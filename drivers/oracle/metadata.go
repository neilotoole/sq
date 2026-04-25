package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the Oracle source.
func getSourceMetadata(ctx context.Context, src *source.Source, db *sql.DB, noSchema bool) (*metadata.Source, error) {
	log := lg.FromContext(ctx)

	md := &metadata.Source{
		Handle:    src.Handle,
		Location:  src.Location,
		Driver:    drivertype.Oracle,
		DBDriver:  drivertype.Oracle,
		DBVersion: "",
		Catalog:   "",
	}

	// Get database version
	// Use v$version instead of v$instance as it's more accessible to regular users
	var version string
	err := db.QueryRowContext(ctx,
		"SELECT BANNER FROM v$version WHERE ROWNUM = 1").Scan(&version)
	if err != nil {
		// If we can't query version (permissions issue), just leave it empty
		md.DBVersion = ""
	} else {
		md.DBVersion = version
	}

	// Get current schema
	var schema string
	err = db.QueryRowContext(ctx,
		"SELECT SYS_CONTEXT('USERENV', 'CURRENT_SCHEMA') FROM DUAL").Scan(&schema)
	if err != nil {
		return nil, errw(err)
	}
	md.Schema = strings.ToLower(schema)
	md.Name = md.Schema
	md.FQName = md.Schema

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
		names = append(names, strings.ToLower(name))
	}

	return names, errw(rows.Err())
}

// getTableMetadata returns metadata for a specific table.
func getTableMetadata(ctx context.Context, db *sql.DB, tblName string) (*metadata.Table, error) {
	_ = progress.FromContext(ctx) // Future: use for progress tracking

	const queryTable = `SELECT
    t.owner,
    t.table_name,
    t.tablespace_name,
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

	var owner, tableName, tablespaceName sql.NullString
	var numRows sql.NullInt64
	var comment sql.NullString
	var bytes int64

	err := db.QueryRowContext(ctx, queryTable, strings.ToUpper(tblName)).Scan(
		&owner, &tableName, &tablespaceName, &numRows, &comment, &bytes)
	if err != nil {
		return nil, errw(err)
	}

	tblMeta := &metadata.Table{
		Name:        strings.ToLower(tblName),
		TableType:   sqlz.TableTypeTable,
		DBTableType: "TABLE",
		RowCount:    numRows.Int64,
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

	tblMeta := &metadata.Table{
		Name:        strings.ToLower(viewName),
		TableType:   sqlz.TableTypeView,
		DBTableType: "VIEW",
		RowCount:    0,
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

	tblMeta := &metadata.Table{
		Name:        strings.ToLower(mvName),
		TableType:   sqlz.TableTypeTable,
		DBTableType: "MATERIALIZED VIEW",
		RowCount:    numRows.Int64,
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
			Name:       strings.ToLower(colName),
			Position:   int64(columnID),
			Kind:       kindFromDBTypeName(lg.FromContext(ctx), colName, dataType),
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
