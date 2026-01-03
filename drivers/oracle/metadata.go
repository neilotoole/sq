package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the Oracle source.
func getSourceMetadata(ctx context.Context, src *source.Source, db *sql.DB, noSchema bool) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:    src.Handle,
		Location:  src.Location,
		Driver:    drivertype.Oracle,
		DBVersion: "",
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
	md.Schema = schema

	if noSchema {
		// Don't fetch schema metadata
		return md, nil
	}

	// Get table metadata
	tables, err := getTablesMetadata(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	md.Tables = tables
	return md, nil
}

// getTablesMetadata returns metadata for all tables in the current schema.
func getTablesMetadata(ctx context.Context, db *sql.DB, _ string) ([]*metadata.Table, error) {
	const query = `SELECT table_name
FROM user_tables
WHERE temporary = 'N'
ORDER BY table_name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tblName string
		if err = rows.Scan(&tblName); err != nil {
			return nil, errw(err)
		}
		tableNames = append(tableNames, tblName)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	// Get metadata for each table
	tables := make([]*metadata.Table, 0, len(tableNames))
	for _, tblName := range tableNames {
		tblMeta, err := getTableMetadata(ctx, db, tblName)
		if err != nil {
			// Log error but continue with other tables
			continue
		}
		tables = append(tables, tblMeta)
	}

	return tables, nil
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
		Name:      tblName,
		TableType: "table",
		RowCount:  numRows.Int64,
		Size:      &bytes,
		Comment:   comment.String,
	}

	// Get column metadata
	cols, err := getColumnsMetadata(ctx, db, tblName)
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
			Name:       colName,
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
