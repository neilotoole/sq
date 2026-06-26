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
	case db.QueryRowContext(
		ctx,
		"SELECT version_full FROM product_component_version WHERE ROWNUM = 1",
	).Scan(&version) == nil && version != "":
		md.DBVersion = version
	case db.QueryRowContext(
		ctx,
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
		if size.Valid {
			md.Size = &size.Int64
		}
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
	}

	// Single classification point for TableCount / ViewCount; materialized
	// views are counted as views inside RecomputeTableCounts.
	md.RecomputeTableCounts()

	// Fetch FKs / unique constraints / indexes in three bulk queries
	// instead of 3N per-table calls inside loadUserSchemaObjectsMetadata.
	// The Assign* helpers route each result to its owning table, and
	// LinkForeignKeys derives FK.Incoming across the whole source.
	allFKs, err := getOracleForeignKeys(ctx, db, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignForeignKeys(log, md.Tables, allFKs)

	allUCs, err := getOracleUniqueConstraints(ctx, db, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignUniqueConstraints(log, md.Tables, allUCs)

	allIdxs, err := getOracleIndexes(ctx, db, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignIndexes(log, md.Tables, allIdxs)

	allChecks, err := getOracleCheckConstraints(ctx, db, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignCheckConstraints(log, md.Tables, allChecks)

	allTriggers, err := getOracleTriggers(ctx, db, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignTriggers(log, md.Tables, allTriggers)

	// View / materialized-view definitions are populated inline by
	// getViewMetadata / getMaterializedViewMetadata, so no source-wide
	// view-definition loader is needed here.

	metadata.LinkForeignKeys(log, md)

	return md, nil
}

// loadUserSchemaObjectsMetadata returns metadata for base tables, materialized
// views, and views in the current schema (USER_* dictionary).
func loadUserSchemaObjectsMetadata(
	ctx context.Context, log *slog.Logger, handle string, db *sql.DB,
) ([]*metadata.Table, error) {
	// Oracle backs every materialized view with a container table of the
	// same name, so that name appears in USER_TABLES as well as USER_MVIEWS.
	// Exclude MV container tables here so the MV is reported once (via
	// getMaterializedViewMetadata below) rather than twice. NOT EXISTS (not
	// NOT IN) avoids the SQL footgun where a NULL in the subquery would
	// filter out every row, and matches the ListTableNames approach.
	baseNames, err := queryOracleObjectNames(ctx, db,
		`SELECT t.table_name FROM user_tables t
WHERE t.temporary = 'N'
  AND NOT EXISTS (
    SELECT 1 FROM user_mviews m
    WHERE m.mview_name = t.table_name
  )
ORDER BY t.table_name`)
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
		tblMeta, err := getTableMetadata(ctx, db, tblName, false)
		if err != nil {
			log.Warn(
				"oracle metadata: skipped base table (continuing)",
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
			log.Warn(
				"oracle metadata: skipped materialized view (continuing)",
				lga.Handle, handle,
				lga.Table, mvName,
				lga.Err, err,
			)
			continue
		}
		out = append(out, tblMeta)
	}

	for _, viewName := range viewNames {
		tblMeta, err := getViewMetadata(ctx, db, viewName, false)
		if err != nil {
			log.Warn(
				"oracle metadata: skipped view (continuing)",
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
		return getViewMetadata(ctx, db, canonical, true)
	case "TABLE":
		return getTableMetadata(ctx, db, canonical, true)
	default:
		return nil, errz.Errorf("unsupported Oracle object type %q for {%s}", objType, name)
	}
}

// getTableMetadata returns metadata for a specific table. The
// loadConstraints flag controls whether per-table FK /
// unique-constraint / index queries are issued:
//
//   - Source-level inspect passes false. [getSourceMetadata] runs
//     three bulk loaders after the table-iteration loop, which is 3
//     round-trips total instead of 3N. [metadata.LinkForeignKeys]
//     then derives [FK.Incoming] across the whole source.
//   - Single-table inspect (grip.TableMetadata) passes true so the
//     standalone [metadata.Table] carries its FK / unique-constraint /
//     index metadata directly, including [FK.Incoming].
func getTableMetadata(ctx context.Context, db *sql.DB, tblName string, loadConstraints bool,
) (*metadata.Table, error) {
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
		&numRows, &comment, &bytes,
	)
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

	// Source-level inspect skips per-table FK / UC / Index queries
	// entirely; [getSourceMetadata] runs three bulk loaders below.
	if !loadConstraints {
		return tblMeta, nil
	}

	outgoing, err := getOracleForeignKeys(ctx, db, tblName)
	if err != nil {
		return nil, err
	}
	incoming, err := getOracleIncomingFKs(ctx, db, tblName)
	if err != nil {
		return nil, err
	}
	tblMeta.FK = metadata.NewFKGroup(outgoing, incoming)

	tblMeta.UniqueConstraints, err = getOracleUniqueConstraints(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.Indexes, err = getOracleIndexes(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.CheckConstraints, err = getOracleCheckConstraints(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.Triggers, err = getOracleTriggers(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	return tblMeta, nil
}

// getOracleUniqueConstraints returns the UNIQUE constraints declared
// on tables in the current Oracle schema. If tblName is empty,
// constraints for every base table are returned; otherwise only
// constraints on tblName are returned. tblName is upper-cased to match
// Oracle's stored identifier convention.
func getOracleUniqueConstraints(ctx context.Context, db *sql.DB, tblName string,
) ([]*metadata.UniqueConstraint, error) {
	log := lg.FromContext(ctx)
	query := `SELECT
    c.constraint_name,
    c.table_name,
    cc.column_name,
    cc.position
FROM user_constraints  c
JOIN user_cons_columns cc
  ON  cc.constraint_name = c.constraint_name
WHERE c.constraint_type = 'U'`
	var args []any
	if tblName != "" {
		query += ` AND c.table_name = :1`
		args = append(args, strings.ToUpper(tblName))
	}
	query += ` ORDER BY c.table_name, c.constraint_name, cc.position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type ucKey struct {
		table, name string
	}
	byKey := map[ucKey]*metadata.UniqueConstraint{}
	var ucs []*metadata.UniqueConstraint
	for rows.Next() {
		var (
			constraintName, ownerTable, columnName string
			position                               int64
		)
		if err = rows.Scan(&constraintName, &ownerTable, &columnName, &position); err != nil {
			return nil, errw(err)
		}
		k := ucKey{table: ownerTable, name: constraintName}
		uc, ok := byKey[k]
		if !ok {
			uc = &metadata.UniqueConstraint{
				Name:  constraintName,
				Table: ownerTable,
			}
			byKey[k] = uc
			ucs = append(ucs, uc)
		}
		uc.Columns = append(uc.Columns, columnName)
	}
	return ucs, errw(rows.Err())
}

// getOracleIndexes returns the physical indexes declared on tables in
// the current Oracle schema. If tblName is empty, indexes for every
// base table are returned. PK-backing indexes are detected via a LEFT
// JOIN against USER_CONSTRAINTS type='P'.
//
// LOB- and system-generated indexes (e.g. on hidden CLOB segments) are
// excluded via index_type not in ('LOB', 'CLUSTER') and a generated=NO
// filter.
func getOracleIndexes(ctx context.Context, db *sql.DB, tblName string) ([]*metadata.Index, error) {
	log := lg.FromContext(ctx)
	query := `SELECT
    i.table_name,
    i.index_name,
    CASE WHEN i.uniqueness = 'UNIQUE' THEN 1 ELSE 0 END AS is_unique,
    CASE WHEN pk.constraint_name IS NOT NULL THEN 1 ELSE 0 END AS is_primary,
    i.index_type,
    ic.column_name,
    ic.column_position
FROM user_indexes      i
JOIN user_ind_columns  ic
  ON  ic.index_name = i.index_name
  AND ic.table_name = i.table_name
LEFT JOIN user_constraints pk
  ON  pk.index_name      = i.index_name
  AND pk.table_name      = i.table_name
  AND pk.constraint_type = 'P'
WHERE i.index_type NOT IN ('LOB', 'CLUSTER')
  AND i.generated = 'N'`
	var args []any
	if tblName != "" {
		query += ` AND i.table_name = :1`
		args = append(args, strings.ToUpper(tblName))
	}
	query += ` ORDER BY i.table_name, i.index_name, ic.column_position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type idxKey struct {
		table, name string
	}
	byKey := map[idxKey]*metadata.Index{}
	var indexes []*metadata.Index
	for rows.Next() {
		var (
			tableName, indexName, indexType, columnName string
			isUnique, isPrimary                         int64
			columnPosition                              int64
		)
		if err = rows.Scan(&tableName, &indexName, &isUnique, &isPrimary,
			&indexType, &columnName, &columnPosition); err != nil {
			return nil, errw(err)
		}
		k := idxKey{table: tableName, name: indexName}
		idx, ok := byKey[k]
		if !ok {
			idx = &metadata.Index{
				Name:    indexName,
				Table:   tableName,
				Unique:  isUnique == 1,
				Primary: isPrimary == 1,
				Type:    indexType,
			}
			byKey[k] = idx
			indexes = append(indexes, idx)
		}
		idx.Columns = append(idx.Columns, columnName)
	}
	return indexes, errw(rows.Err())
}

// getOracleForeignKeys returns the outgoing foreign-key constraints
// declared on tables in the current Oracle schema. If tblName is empty,
// FKs for every base table in the schema are returned; otherwise only
// FKs declared on tblName are returned. tblName is matched after
// upper-casing, since Oracle stores unquoted identifiers in upper case.
//
// Oracle exposes only the DELETE_RULE on USER_CONSTRAINTS; there is no
// ON UPDATE referential action, so [ForeignKey.OnUpdate] is left empty
// for this driver. Cross-table linking is not done here; callers must
// invoke metadata.LinkForeignKeys at the source level.
func getOracleForeignKeys(ctx context.Context, db *sql.DB, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)
	// USER_CONSTRAINTS rows of type 'R' are referential constraints (FKs).
	// R_CONSTRAINT_NAME identifies the referenced unique/PK constraint;
	// joining USER_CONS_COLUMNS twice on matching POSITION lines up the
	// FK column with its corresponding parent column for composite keys.
	// NULLIF clears ref_schema when the reference is in the current
	// user's schema, matching the normalization that
	// metadata.LinkForeignKeys applies at the source level.
	query := `SELECT
    c.constraint_name,
    c.table_name                                                       AS fk_table,
    fkc.column_name                                                    AS fk_column,
    fkc.position                                                       AS ordinal_position,
    NULLIF(c.r_owner, SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA'))        AS ref_schema,
    pkc.table_name                                                     AS ref_table,
    pkc.column_name                                                    AS ref_column,
    c.delete_rule
FROM user_constraints  c
JOIN user_cons_columns fkc
  ON  fkc.constraint_name = c.constraint_name
JOIN user_cons_columns pkc
  ON  pkc.constraint_name = c.r_constraint_name
  AND pkc.position        = fkc.position
WHERE c.constraint_type = 'R'`
	var args []any
	if tblName != "" {
		query += ` AND c.table_name = :1`
		args = append(args, strings.ToUpper(tblName))
	}
	query += ` ORDER BY c.table_name, c.constraint_name, fkc.position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type fkKey struct {
		table, name string
	}
	byKey := map[fkKey]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		var (
			constraintName, fkTable, fkColumn string
			refTable, refCol                  string
			refSchema, deleteRule             sql.NullString
			ordinalPosition                   int64
		)
		if err = rows.Scan(&constraintName, &fkTable, &fkColumn, &ordinalPosition,
			&refSchema, &refTable, &refCol, &deleteRule); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, name: constraintName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:      constraintName,
				Table:     fkTable,
				RefSchema: refSchema.String,
				RefTable:  refTable,
				OnDelete:  deleteRule.String,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return fks, nil
}

// getOracleIncomingFKs returns the foreign-key constraints declared on
// other tables in the current schema whose referenced side is tblName.
// USER_CONSTRAINTS scopes results to the current user's schema, so
// cross-schema references from tables in other schemas are not
// reported here.
func getOracleIncomingFKs(ctx context.Context, db *sql.DB, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT
    c.constraint_name,
    c.table_name      AS fk_table,
    fkc.column_name   AS fk_column,
    fkc.position      AS ordinal_position,
    pk.table_name     AS ref_table,
    pkc.column_name   AS ref_column,
    c.delete_rule
FROM user_constraints  c
JOIN user_constraints  pk
  ON  pk.constraint_name = c.r_constraint_name
JOIN user_cons_columns fkc
  ON  fkc.constraint_name = c.constraint_name
JOIN user_cons_columns pkc
  ON  pkc.constraint_name = c.r_constraint_name
  AND pkc.position        = fkc.position
WHERE c.constraint_type = 'R'
  AND pk.table_name = :1
ORDER BY c.table_name, c.constraint_name, fkc.position`

	rows, err := db.QueryContext(ctx, query, strings.ToUpper(tblName))
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type fkKey struct {
		table, name string
	}
	byKey := map[fkKey]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		var (
			constraintName, fkTable, fkColumn string
			refTable, refCol                  string
			deleteRule                        sql.NullString
			ordinalPosition                   int64
		)
		if err = rows.Scan(&constraintName, &fkTable, &fkColumn, &ordinalPosition,
			&refTable, &refCol, &deleteRule); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, name: constraintName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:     constraintName,
				Table:    fkTable,
				RefTable: refTable,
				OnDelete: deleteRule.String,
			}
			// RefSchema intentionally left empty: the referenced table
			// is the current one we're inspecting, which lives in the
			// current schema by USER_CONSTRAINTS scoping.
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	return fks, errw(rows.Err())
}

// getOracleCheckConstraints returns the CHECK constraints declared on
// tables in the current Oracle schema. If tblName is empty, checks for
// every base table are returned; otherwise only checks on tblName.
//
// SEARCH_CONDITION_VC (a VARCHAR2 mirror of the LONG SEARCH_CONDITION,
// available since 12.1) supplies the clause, sidestepping LONG entirely.
//
// Oracle models a NOT NULL column constraint as a system-generated CHECK
// of the form "COL" IS NOT NULL. Those are not user-authored CHECKs, so
// they are filtered out via the SEARCH_CONDITION_VC NOT LIKE '%IS NOT
// NULL' predicate. A genuine user CHECK whose clause happens to end in
// "IS NOT NULL" would also be excluded; that ambiguity is inherent to
// Oracle's dictionary (NOT NULL and a CHECK(... IS NOT NULL) are stored
// identically) and is an accepted limitation.
func getOracleCheckConstraints(ctx context.Context, db *sql.DB, tblName string,
) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)
	query := `SELECT
    c.table_name,
    c.constraint_name,
    c.search_condition_vc
FROM user_constraints c
WHERE c.constraint_type = 'C'
  AND c.search_condition_vc IS NOT NULL
  AND c.search_condition_vc NOT LIKE '%IS NOT NULL'`
	var args []any
	if tblName != "" {
		query += ` AND c.table_name = :1`
		args = append(args, strings.ToUpper(tblName))
	}
	query += ` ORDER BY c.table_name, c.constraint_name`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var checks []*metadata.CheckConstraint
	for rows.Next() {
		cc := &metadata.CheckConstraint{}
		if err = rows.Scan(&cc.Table, &cc.Name, &cc.Clause); err != nil {
			return nil, errw(err)
		}
		checks = append(checks, cc)
	}
	return checks, errw(rows.Err())
}

// getOracleTriggers returns the triggers declared on tables (and views,
// for INSTEAD OF triggers) in the current Oracle schema. If tblName is
// empty, triggers for every relation are returned; otherwise only those
// on tblName.
//
// TRIGGER_TYPE (e.g. "BEFORE EACH ROW", "AFTER STATEMENT", "INSTEAD OF")
// is parsed to a canonical Timing. TRIGGERING_EVENT (e.g. "INSERT OR
// UPDATE") is split on " OR " into Events. STATUS maps to Enabled.
// TRIGGER_BODY is a LONG column carrying the trigger PL/SQL; go-ora reads
// it cleanly, so it is captured into Definition.
func getOracleTriggers(ctx context.Context, db *sql.DB, tblName string,
) ([]*metadata.Trigger, error) {
	log := lg.FromContext(ctx)
	query := `SELECT
    table_name,
    trigger_name,
    trigger_type,
    triggering_event,
    status,
    trigger_body
FROM user_triggers`
	var args []any
	if tblName != "" {
		query += ` WHERE table_name = :1`
		args = append(args, strings.ToUpper(tblName))
	}
	query += ` ORDER BY table_name, trigger_name`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var triggers []*metadata.Trigger
	for rows.Next() {
		var (
			tableName, trigName, trigType, trigEvent, status string
			body                                             sql.NullString
		)
		if err = rows.Scan(&tableName, &trigName, &trigType, &trigEvent,
			&status, &body); err != nil {
			return nil, errw(err)
		}

		var timing string
		switch upper := strings.ToUpper(trigType); {
		case strings.HasPrefix(upper, "INSTEAD OF"):
			timing = "INSTEAD OF"
		case strings.HasPrefix(upper, "BEFORE"):
			timing = "BEFORE"
		case strings.HasPrefix(upper, "AFTER"):
			timing = "AFTER"
		}

		// TRIGGERING_EVENT is a space-delimited " OR " list, e.g.
		// "INSERT OR UPDATE OR DELETE". Oracle 23c stores the plain keyword
		// ("UPDATE") even for column-scoped triggers, but defensively normalize
		// any "UPDATE OF col1, col2" form to its base keyword by taking only
		// the leading word. Deduplicate in case normalization produces repeats.
		var events []string
		seen := make(map[string]bool)
		for _, e := range strings.Split(strings.ToUpper(trigEvent), " OR ") {
			e = strings.TrimSpace(e)
			if e == "" {
				continue
			}
			// Collapse "UPDATE OF col1, col2" → "UPDATE" (take leading keyword).
			if i := strings.IndexByte(e, ' '); i > 0 {
				e = e[:i]
			}
			if !seen[e] {
				seen[e] = true
				events = append(events, e)
			}
		}

		// Allocate a fresh bool per row so each Enabled points to its own
		// value rather than a shared loop variable.
		enabled := strings.EqualFold(status, "ENABLED")
		triggers = append(triggers, &metadata.Trigger{
			Name:       trigName,
			Table:      tableName,
			Timing:     timing,
			Events:     events,
			Enabled:    &enabled,
			Definition: strings.TrimSpace(body.String),
		})
	}
	return triggers, errw(rows.Err())
}

// getOracleViewDefinition returns the defining SQL text for the named view
// from USER_VIEWS.TEXT (a LONG column, read cleanly by go-ora). An empty
// string is returned when the view has no recorded text.
func getOracleViewDefinition(ctx context.Context, db *sql.DB, viewName string) (string, error) {
	var text sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT text FROM user_views WHERE view_name = :1`,
		strings.ToUpper(viewName)).Scan(&text)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", errw(err)
	}
	return strings.TrimSpace(text.String), nil
}

// getViewMetadata returns metadata for a view (USER_VIEWS / USER_TAB_COLUMNS).
//
// loadTriggers controls whether per-view trigger metadata is fetched:
//   - Pass true for the per-table path (getObjectMetadata / Grip.TableMetadata)
//     so that the standalone [metadata.Table] carries its INSTEAD OF triggers.
//   - Pass false for the source-wide path (loadUserSchemaObjectsMetadata) where
//     a bulk getOracleTriggers("") + AssignTriggers runs after this function
//     returns and would overwrite any per-view result; fetching triggers here
//     would be an unnecessary N extra round-trips.
func getViewMetadata(ctx context.Context, db *sql.DB, viewName string, loadTriggers bool) (*metadata.Table, error) {
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

	// ViewDefinition is set inline here (rather than via a separate
	// source-wide loader) so that both single-table inspect and
	// full-source inspect populate it uniformly; the extra dictionary
	// round-trip is negligible next to the liveRowCount COUNT(*) above.
	if tblMeta.ViewDefinition, err = getOracleViewDefinition(ctx, db, viewName); err != nil {
		return nil, err
	}

	// INSTEAD OF triggers are stored in USER_TRIGGERS with TABLE_NAME equal
	// to the view name, so getOracleTriggers returns them when called for a
	// view.  Only fetch here for the per-table path; the source-wide path
	// relies on a subsequent bulk AssignTriggers call that overwrites this.
	if loadTriggers {
		tblMeta.Triggers, err = getOracleTriggers(ctx, db, viewName)
		if err != nil {
			return nil, err
		}
	}

	return tblMeta, nil
}

// getMaterializedViewMetadata returns metadata for a materialized view.
func getMaterializedViewMetadata(ctx context.Context, db *sql.DB, mvName string) (*metadata.Table, error) {
	// USER_MVIEWS has no NUM_ROWS column; the CBO row-count statistic for a
	// materialized view lives on its container table in USER_TABLES (joined
	// here by name). It's NULL until stats are gathered, in which case the
	// liveRowCount fallback below applies, mirroring getTableMetadata.
	const q = `SELECT m.mview_name, tc.comments, t.num_rows,
    NVL(s.bytes, 0) AS bytes
FROM user_mviews m
LEFT JOIN user_tables t
    ON m.mview_name = t.table_name
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

	// NUM_ROWS here comes from the MV's container table in USER_TABLES (see
	// the query above); like any USER_TABLES.NUM_ROWS it is CBO-stats-derived
	// and NULL until DBMS_STATS / ANALYZE has run on the materialized view.
	// See getTableMetadata for the full rationale; the same fallback applies
	// here.
	rowCount := numRows.Int64
	if !numRows.Valid {
		var err error
		if rowCount, err = liveRowCount(ctx, db, mvName); err != nil {
			return nil, err
		}
	}

	tblMeta := &metadata.Table{
		Name:        mvName,
		TableType:   sqlz.TableTypeMaterializedView,
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

	// USER_MVIEWS.QUERY is a LONG column holding the materialized view's
	// defining SELECT; go-ora reads it cleanly. Fetched in its own simple
	// query (rather than joined into the metadata query above) to keep the
	// LONG read away from the aggregate/segment joins.
	var query sql.NullString
	if err = db.QueryRowContext(ctx,
		`SELECT query FROM user_mviews WHERE mview_name = :1`,
		strings.ToUpper(mvName)).Scan(&query); err != nil {
		return nil, errw(err)
	}
	tblMeta.ViewDefinition = strings.TrimSpace(query.String)

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
	// USER_TAB_COLS (not USER_TAB_COLUMNS) is the dictionary view that
	// exposes the IDENTITY_COLUMN / VIRTUAL_COLUMN / COLLATION flags. It
	// also lists hidden/system columns (e.g. the shadow column backing an
	// identity sequence, or function-based index expression columns), so
	// HIDDEN_COLUMN = 'NO' restricts the result to the user-declared
	// columns that USER_TAB_COLUMNS would have returned.
	//
	// DATA_DEFAULT is a LONG column; for a virtual column it holds the
	// generation expression (mapped to GeneratedExpr below). go-ora reads
	// it cleanly, so no TO_LOB/CLOB cast is needed.
	const query = `SELECT
    c.column_name,
    c.data_type,
    c.data_length,
    c.data_precision,
    c.data_scale,
    c.nullable,
    c.column_id,
    c.identity_column,
    c.virtual_column,
    c.collation,
    c.data_default,
    cc.comments
FROM user_tab_cols c
LEFT JOIN user_col_comments cc
    ON c.table_name = cc.table_name
    AND c.column_name = cc.column_name
WHERE c.table_name = :1
  AND c.hidden_column = 'NO'
ORDER BY c.column_id`

	// Collect the table's primary-key column names up front in one
	// query so the scan loop can flag PK columns via a map lookup
	// instead of issuing one COUNT(*) per column (a per-column round
	// trip was the previous shape — quadratic on wide tables).
	pkCols, err := getOraclePKColumnNames(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

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
		var identityCol, virtualCol string
		var collation, dataDefault, comment sql.NullString

		err = rows.Scan(&colName, &dataType, &dataLength, &dataPrecision,
			&dataScale, &nullable, &columnID, &identityCol, &virtualCol,
			&collation, &dataDefault, &comment)
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
			PrimaryKey: pkCols[colName],
			// Oracle models auto-increment exclusively as IDENTITY, so
			// AutoIncrement is left false; Identity carries the signal.
			Identity:  identityCol == "YES",
			Generated: virtualCol == "YES",
			Collation: collation.String,
		}
		// DATA_DEFAULT holds the generation expression for a virtual
		// (generated) column; for ordinary columns it is the DEFAULT
		// clause, which sq does not surface for Oracle, so only capture
		// it as GeneratedExpr when the column is virtual.
		if col.Generated {
			col.GeneratedExpr = strings.TrimSpace(dataDefault.String)
		}

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// getOraclePKColumnNames returns the primary-key column names for
// tblName as a set keyed by the column name. Returns an empty
// (non-nil) map when the table has no primary key.
func getOraclePKColumnNames(ctx context.Context, db *sql.DB, tblName string) (map[string]bool, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT cols.column_name
FROM user_constraints cons
INNER JOIN user_cons_columns cols
    ON cons.constraint_name = cols.constraint_name
WHERE cons.table_name = :1
  AND cons.constraint_type = 'P'`

	rows, err := db.QueryContext(ctx, query, strings.ToUpper(tblName))
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	pkCols := map[string]bool{}
	for rows.Next() {
		var col string
		if err = rows.Scan(&col); err != nil {
			return nil, errw(err)
		}
		pkCols[col] = true
	}
	return pkCols, errw(rows.Err())
}
