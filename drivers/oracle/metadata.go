package oracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the Oracle source.
func getSourceMetadata(ctx context.Context, src *source.Source, db *sql.DB, noSchema bool) (*metadata.Source, error) {
	log := lg.FromContext(ctx)

	md := &metadata.Source{
		Handle:   src.Handle,
		Location: src.Location,
		Driver:   drivertype.Oracle,
		DBDriver: drivertype.Oracle,
	}

	// One round-trip for the SYS_CONTEXT scalars: schema, session user, and
	// database name (which serves as the catalog — see Renderer's catalog()
	// override).
	const summaryQuery = `SELECT
    SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA'),
    SYS_CONTEXT('USERENV', 'SESSION_USER'),
    SYS_CONTEXT('USERENV', 'DB_NAME')
FROM DUAL`
	var schema, user, catalog string
	if err := db.QueryRowContext(ctx, summaryQuery).Scan(&schema, &user, &catalog); err != nil {
		return nil, errw(err)
	}
	md.Schema = schema
	md.User = user
	md.Catalog = catalog
	md.Name = md.Schema
	// Use the 3-part catalog.schema.name form when DB_NAME is available
	// (it always is in modern Oracle, multitenant or non-CDB), matching
	// the Postgres / SQL Server convention. Fall back to schema-only on
	// the unlikely event that catalog is empty.
	if md.Catalog != "" {
		md.FQName = md.Catalog + "." + md.Schema
	} else {
		md.FQName = md.Schema
	}

	// DBProduct is the descriptive banner (e.g. "Oracle Database 23ai
	// Free Release ..."). DBVersion prefers v$instance.version (numeric,
	// e.g. "23.26.1.0.0") and falls back to the banner if v$instance is
	// not readable.
	var banner string
	if err := db.QueryRowContext(ctx,
		"SELECT BANNER FROM v$version WHERE ROWNUM = 1").Scan(&banner); err == nil {
		md.DBProduct = banner
	}
	// Version preference order:
	//   1. PRODUCT_COMPONENT_VERSION.VERSION_FULL — patch-level version
	//      (e.g. "23.26.1.0.0"), readable by every user.
	//   2. V$INSTANCE.VERSION — clean numeric version, but only DBAs can
	//      see V$ views.
	//   3. The BANNER as a last resort.
	var version string
	switch {
	case db.QueryRowContext(ctx,
		"SELECT version_full FROM product_component_version WHERE ROWNUM = 1",
	).Scan(&version) == nil && version != "":
		md.DBVersion = version
	case db.QueryRowContext(ctx,
		"SELECT version FROM v$instance WHERE ROWNUM = 1",
	).Scan(&version) == nil && version != "":
		md.DBVersion = version
	default:
		md.DBVersion = banner
	}

	// Size: total bytes of segments owned by the connected user (tables,
	// indexes, LOBs, etc.). USER_SEGMENTS is readable by every user; the
	// PDB- or database-wide equivalents (DBA_DATA_FILES) require DBA
	// privileges and aren't appropriate for an ordinary application
	// account. NVL guards against an empty user with no segments.
	var size sql.NullInt64
	if err := db.QueryRowContext(ctx,
		"SELECT NVL(SUM(bytes), 0) FROM user_segments").Scan(&size); err == nil {
		md.Size = size.Int64
	}

	// DBProperties surfaces driver-level session/version values via the
	// shared SQLDriver helper.
	props, err := getDBProperties(ctx, db)
	if err != nil {
		return nil, err
	}
	md.DBProperties = props

	if noSchema {
		// Don't fetch schema metadata
		return md, nil
	}

	tables, err := loadUserSchemaObjectsMetadata(ctx, log, src.Handle, db)
	if err != nil {
		return nil, err
	}

	md.Tables = tables
	for _, tbl := range md.Tables {
		// 3-part catalog.schema.name when catalog (DB_NAME) is available,
		// matching the Postgres / SQL Server convention; 2-part schema.name
		// fallback otherwise.
		if md.Catalog != "" {
			tbl.FQName = md.Catalog + "." + md.Schema + "." + tbl.Name
		} else {
			tbl.FQName = md.Schema + "." + tbl.Name
		}
		switch tbl.TableType {
		case sqlz.TableTypeTable:
			md.TableCount++
		case sqlz.TableTypeView:
			md.ViewCount++
		}
	}

	return md, nil
}

// loadUserSchemaObjectsMetadata returns metadata for base tables, materialized
// views, and views in the current schema (USER_* dictionary).
func loadUserSchemaObjectsMetadata(
	ctx context.Context, log *slog.Logger, handle string, db *sql.DB,
) ([]*metadata.Table, error) {
	baseNames, err := queryOracleObjectNames(ctx, db,
		`SELECT table_name FROM user_tables WHERE temporary = 'N' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}

	mviewNames, err := queryOracleObjectNames(ctx, db,
		`SELECT mview_name FROM user_mviews ORDER BY mview_name`)
	if err != nil {
		return nil, err
	}

	viewNames, err := queryOracleObjectNames(ctx, db,
		`SELECT view_name FROM user_views ORDER BY view_name`)
	if err != nil {
		return nil, err
	}

	nCap := len(baseNames) + len(mviewNames) + len(viewNames)
	out := make([]*metadata.Table, 0, nCap)

	for _, tblName := range baseNames {
		tblMeta, err := getTableMetadata(ctx, db, tblName)
		if err != nil {
			log.Warn("oracle metadata: skipped base table (continuing)",
				lga.Handle, handle,
				lga.Table, tblName,
				lga.Err, err,
			)
			continue
		}
		out = append(out, tblMeta)
	}

	for _, mvName := range mviewNames {
		tblMeta, err := getMaterializedViewMetadata(ctx, db, mvName)
		if err != nil {
			log.Warn("oracle metadata: skipped materialized view (continuing)",
				lga.Handle, handle,
				lga.Table, mvName,
				lga.Err, err,
			)
			continue
		}
		out = append(out, tblMeta)
	}

	for _, viewName := range viewNames {
		tblMeta, err := getViewMetadata(ctx, db, viewName)
		if err != nil {
			log.Warn("oracle metadata: skipped view (continuing)",
				lga.Handle, handle,
				lga.Table, viewName,
				lga.Err, err,
			)
			continue
		}
		out = append(out, tblMeta)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out, nil
}

func queryOracleObjectNames(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}
		names = append(names, name)
	}

	return names, errw(rows.Err())
}

// getObjectMetadata returns metadata for a single named schema object,
// classifying it via USER_OBJECTS and dispatching to the appropriate
// table/view/materialized-view loader. Names are case-insensitive (Oracle
// stores unquoted identifiers as upper case).
//
// When an object name resolves to both a TABLE row and a MATERIALIZED VIEW
// row (Oracle backs an MV with a base table of the same name), the MV
// branch is preferred so that callers see the MV semantics.
func getObjectMetadata(ctx context.Context, db *sql.DB, name string) (*metadata.Table, error) {
	const q = `SELECT object_type FROM user_objects
WHERE object_name = :1 AND object_type IN ('TABLE', 'VIEW', 'MATERIALIZED VIEW')
ORDER BY CASE object_type
    WHEN 'MATERIALIZED VIEW' THEN 1
    WHEN 'VIEW' THEN 2
    WHEN 'TABLE' THEN 3
END
FETCH FIRST 1 ROW ONLY`

	// Canonicalize to Oracle's stored case (upper for unquoted identifiers)
	// so that the returned metadata's Name field reflects the database's
	// actual identifier rather than echoing the caller's input case.
	canonical := strings.ToUpper(name)

	var objType string
	err := db.QueryRowContext(ctx, q, canonical).Scan(&objType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errz.Errorf("table or view {%s} does not exist", name)
		}
		return nil, errw(err)
	}

	switch objType {
	case "MATERIALIZED VIEW":
		return getMaterializedViewMetadata(ctx, db, canonical)
	case "VIEW":
		return getViewMetadata(ctx, db, canonical)
	case "TABLE":
		return getTableMetadata(ctx, db, canonical)
	default:
		return nil, errz.Errorf("unsupported Oracle object type %q for {%s}", objType, name)
	}
}

// getTableMetadata returns metadata for a specific table.
func getTableMetadata(ctx context.Context, db *sql.DB, tblName string) (*metadata.Table, error) {
	_ = progress.FromContext(ctx) // Future: use for progress tracking

	// USER_TABLES is scoped to the current user, so it has no OWNER column;
	// querying t.owner here previously raised ORA-00904.
	const queryTable = `SELECT
    t.num_rows,
    tc.comments,
    NVL(s.bytes, 0) AS bytes
FROM user_tables t
LEFT JOIN user_tab_comments tc
    ON t.table_name = tc.table_name
    AND tc.table_type = 'TABLE'
LEFT JOIN (
    SELECT segment_name, SUM(bytes) AS bytes
    FROM user_segments
    WHERE segment_type = 'TABLE'
    GROUP BY segment_name
) s ON t.table_name = s.segment_name
WHERE t.table_name = :1`

	var numRows sql.NullInt64
	var comment sql.NullString
	var bytes int64

	err := db.QueryRowContext(ctx, queryTable, strings.ToUpper(tblName)).Scan(
		&numRows, &comment, &bytes)
	if err != nil {
		return nil, errw(err)
	}

	// USER_TABLES.NUM_ROWS is a Cost-Based Optimizer statistics column, not a
	// live row count. Oracle populates it only when statistics are gathered
	// (DBMS_STATS.GATHER_TABLE_STATS, ANALYZE TABLE, or the auto-stats job);
	// for freshly loaded schemas (e.g. a freshly seeded Sakila container)
	// the column is NULL and would otherwise scan as zero. Other sq drivers
	// (Postgres, MySQL, SQLite, …) report live counts in source/table
	// metadata, so when NUM_ROWS is NULL we fall back to SELECT COUNT(*) to
	// match that contract. When stats *do* exist we trust them, even if
	// stale: gathering vs. recomputing is a DBA-controlled tradeoff and a
	// full COUNT(*) on every metadata fetch would be unacceptably expensive
	// on large tables.
	rowCount := numRows.Int64
	if !numRows.Valid {
		if rowCount, err = liveRowCount(ctx, db, tblName); err != nil {
			return nil, err
		}
	}

	tblMeta := &metadata.Table{
		Name:        tblName,
		TableType:   sqlz.TableTypeTable,
		DBTableType: "TABLE",
		RowCount:    rowCount,
		Size:        &bytes,
		Comment:     comment.String,
	}

	// Get column metadata
	cols, err := getColumnsMetadata(ctx, db, tblName)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	return tblMeta, nil
}

// getViewMetadata returns metadata for a view (USER_VIEWS / USER_TAB_COLUMNS).
func getViewMetadata(ctx context.Context, db *sql.DB, viewName string) (*metadata.Table, error) {
	const q = `SELECT v.view_name, tc.comments
FROM user_views v
LEFT JOIN user_tab_comments tc
    ON v.view_name = tc.table_name
    AND tc.table_type = 'VIEW'
WHERE v.view_name = :1`

	var name string
	var comment sql.NullString
	if err := db.QueryRowContext(ctx, q, strings.ToUpper(viewName)).Scan(&name, &comment); err != nil {
		return nil, errw(err)
	}

	// Views have no data-dictionary row count (USER_VIEWS doesn't carry
	// one; the view is virtual). Match the behavior of other drivers
	// (e.g. Postgres) by running a live COUNT(*) so `sq inspect` reports
	// the actual cardinality the user would see when querying the view.
	rowCount, err := liveRowCount(ctx, db, viewName)
	if err != nil {
		return nil, err
	}

	tblMeta := &metadata.Table{
		Name:        viewName,
		TableType:   sqlz.TableTypeView,
		DBTableType: "VIEW",
		RowCount:    rowCount,
		Size:        nil,
		Comment:     comment.String,
	}

	cols, err := getColumnsMetadata(ctx, db, viewName)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	return tblMeta, nil
}

// getMaterializedViewMetadata returns metadata for a materialized view.
func getMaterializedViewMetadata(ctx context.Context, db *sql.DB, mvName string) (*metadata.Table, error) {
	const q = `SELECT m.mview_name, tc.comments, m.num_rows,
    NVL(s.bytes, 0) AS bytes
FROM user_mviews m
LEFT JOIN user_tab_comments tc
    ON m.mview_name = tc.table_name
    AND tc.table_type = 'MATERIALIZED VIEW'
LEFT JOIN (
    SELECT segment_name, SUM(bytes) AS bytes
    FROM user_segments
    WHERE segment_type IN ('TABLE', 'MATERIALIZED VIEW')
    GROUP BY segment_name
) s ON m.mview_name = s.segment_name
WHERE m.mview_name = :1`

	var name string
	var comment sql.NullString
	var numRows sql.NullInt64
	var bytes int64
	if err := db.QueryRowContext(ctx, q, strings.ToUpper(mvName)).Scan(&name, &comment, &numRows, &bytes); err != nil {
		return nil, errw(err)
	}

	// USER_MVIEWS.NUM_ROWS, like USER_TABLES.NUM_ROWS, is CBO-stats-derived
	// and is NULL until DBMS_STATS / ANALYZE has run on the materialized
	// view. See getTableMetadata for the full rationale; the same fallback
	// applies here.
	rowCount := numRows.Int64
	if !numRows.Valid {
		var err error
		if rowCount, err = liveRowCount(ctx, db, mvName); err != nil {
			return nil, err
		}
	}

	tblMeta := &metadata.Table{
		Name:        mvName,
		TableType:   sqlz.TableTypeTable,
		DBTableType: "MATERIALIZED VIEW",
		RowCount:    rowCount,
		Comment:     comment.String,
	}
	if bytes > 0 {
		tblMeta.Size = &bytes
	}

	cols, err := getColumnsMetadata(ctx, db, mvName)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	return tblMeta, nil
}

// liveRowCount returns SELECT COUNT(*) for tblName. It exists as a fallback
// path because Oracle's data-dictionary row counts (USER_TABLES.NUM_ROWS,
// USER_MVIEWS.NUM_ROWS) are CBO statistics, populated only after stats are
// gathered, and are NULL otherwise. tblName is expected to be the canonical
// Oracle identifier as stored in the data dictionary (uppercase for
// unquoted identifiers); it's re-quoted defensively to handle any
// mixed-case input.
func liveRowCount(ctx context.Context, db *sql.DB, tblName string) (int64, error) {
	quoted := stringz.DoubleQuote(strings.ToUpper(tblName))
	var count int64
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoted).Scan(&count); err != nil {
		return 0, errw(err)
	}
	return count, nil
}

// getColumnsMetadata returns metadata for all columns in a table.
func getColumnsMetadata(ctx context.Context, db *sql.DB, tblName string) ([]*metadata.Column, error) {
	const query = `SELECT
    c.column_name,
    c.data_type,
    c.data_length,
    c.data_precision,
    c.data_scale,
    c.nullable,
    c.column_id,
    c.data_default,
    cc.comments
FROM user_tab_columns c
LEFT JOIN user_col_comments cc
    ON c.table_name = cc.table_name
    AND c.column_name = cc.column_name
WHERE c.table_name = :1
ORDER BY c.column_id`

	rows, err := db.QueryContext(ctx, query, strings.ToUpper(tblName))
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var cols []*metadata.Column
	for rows.Next() {
		var colName, dataType, nullable string
		var dataLength sql.NullInt64
		var dataPrecision, dataScale sql.NullInt64
		var columnID int
		var dataDefault, comment sql.NullString

		err = rows.Scan(&colName, &dataType, &dataLength, &dataPrecision,
			&dataScale, &nullable, &columnID, &dataDefault, &comment)
		if err != nil {
			return nil, errw(err)
		}

		// Build full type name
		fullTypeName := dataType
		if dataPrecision.Valid {
			if dataScale.Valid && dataScale.Int64 > 0 {
				fullTypeName = fmt.Sprintf("%s(%d,%d)", dataType, dataPrecision.Int64, dataScale.Int64)
			} else if dataPrecision.Int64 > 0 {
				fullTypeName = fmt.Sprintf("%s(%d)", dataType, dataPrecision.Int64)
			}
		} else if dataLength.Valid && dataLength.Int64 > 0 {
			fullTypeName = fmt.Sprintf("%s(%d)", dataType, dataLength.Int64)
		}

		col := &metadata.Column{
			Name:       colName,
			Position:   int64(columnID),
			Kind:       kindFromDBTypeName(lg.FromContext(ctx), colName, fullTypeName),
			ColumnType: fullTypeName,
			Nullable:   nullable == "Y",
			Comment:    comment.String,
		}

		// Check if primary key
		isPK, err := isColumnPrimaryKey(ctx, db, tblName, colName)
		if err != nil {
			return nil, err
		}
		col.PrimaryKey = isPK

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// isColumnPrimaryKey checks if a column is part of a primary key.
func isColumnPrimaryKey(ctx context.Context, db *sql.DB, tblName, colName string) (bool, error) {
	const query = `SELECT COUNT(*)
FROM user_constraints cons
INNER JOIN user_cons_columns cols
    ON cons.constraint_name = cols.constraint_name
WHERE cons.table_name = :1
  AND cols.column_name = :2
  AND cons.constraint_type = 'P'`

	var count int
	err := db.QueryRowContext(ctx, query,
		strings.ToUpper(tblName),
		strings.ToUpper(colName)).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}
