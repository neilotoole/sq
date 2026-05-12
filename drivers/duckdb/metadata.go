package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// SQL queries for DuckDB system catalog introspection.
const (
	// stmtVersion returns the DuckDB version string, e.g. "v1.5.2".
	stmtVersion = `SELECT version()`

	// stmtCurrentSchema returns the name of the current schema.
	stmtCurrentSchema = `SELECT current_schema()`

	// stmtCurrentCatalog returns the name of the current catalog (database).
	stmtCurrentCatalog = `SELECT current_database()`

	// stmtSchemas lists user-visible schemas in the current catalog,
	// excluding DuckDB system schemas.
	stmtSchemas = `SELECT schema_name, catalog_name, schema_owner
FROM information_schema.schemata
WHERE catalog_name = current_database()
AND schema_name NOT IN ('information_schema', 'pg_catalog')
ORDER BY schema_name`

	// stmtSchemasNames lists only schema names in the current catalog.
	stmtSchemaNames = `SELECT schema_name
FROM information_schema.schemata
WHERE catalog_name = current_database()
AND schema_name NOT IN ('information_schema', 'pg_catalog')
ORDER BY schema_name`

	// stmtSchemaExists checks whether a named schema exists in the current catalog.
	stmtSchemaExists = `SELECT COUNT(schema_name)
FROM information_schema.schemata
WHERE catalog_name = current_database()
AND schema_name = ?`

	// stmtTables lists all user tables and views in a given schema.
	// Both duckdb_tables() and duckdb_views() are used so that the
	// schema-scoped table_type label is consistent across the result set.
	stmtTables = `SELECT schema_name, table_name, 'TABLE' AS table_type, comment
FROM duckdb_tables()
WHERE NOT internal AND schema_name = ?
UNION ALL
SELECT schema_name, view_name, 'VIEW', comment
FROM duckdb_views()
WHERE NOT internal AND schema_name = ?
ORDER BY table_name`

	// stmtTableNames lists table (and optionally view) names in a given schema.
	// The caller constructs the full query using buildTableNamesQuery.

	// stmtColumns lists columns for a given (schema, table), ordered by position.
	stmtColumns = `SELECT column_name, column_index, column_default, is_nullable, data_type, comment
FROM duckdb_columns()
WHERE schema_name = ? AND table_name = ?
ORDER BY column_index`

	// stmtPrimaryKeys returns the primary-key column names for a given table.
	stmtPrimaryKeys = `SELECT CAST(constraint_column_names AS VARCHAR)
FROM duckdb_constraints()
WHERE schema_name = ? AND table_name = ? AND constraint_type = 'PRIMARY KEY'
LIMIT 1`
)

// getSourceMetadata builds a *metadata.Source for src using db.
// When noSchema is true, column-level metadata is skipped for each table
// (used by "sq inspect --overview").
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:   src.Handle,
		Location: src.Location,
		Driver:   src.Type,
		DBDriver: src.Type,
	}

	// Fetch DuckDB version.
	var ver string
	if err := db.QueryRowContext(ctx, stmtVersion).Scan(&ver); err != nil {
		return nil, errz.Err(err)
	}
	md.DBVersion = strings.TrimPrefix(ver, "v")
	md.DBProduct = "DuckDB " + md.DBVersion

	// Fetch current catalog and schema.
	if err := db.QueryRowContext(ctx, stmtCurrentCatalog).Scan(&md.Name); err != nil {
		return nil, errz.Err(err)
	}
	md.Catalog = md.Name

	var schema string
	if err := db.QueryRowContext(ctx, stmtCurrentSchema).Scan(&schema); err != nil {
		return nil, errz.Err(err)
	}
	md.Schema = schema
	md.FQName = md.Name + "." + schema

	if noSchema {
		// Caller only wants catalog-level info; skip per-table enumeration.
		return md, nil
	}

	// Enumerate tables and views in the current schema.
	tblMetas, err := getSchemaTableMetadata(ctx, db, schema)
	if err != nil {
		return nil, err
	}

	md.Tables = tblMetas
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

// getSchemaTableMetadata returns metadata for every table/view in schemaName.
func getSchemaTableMetadata(ctx context.Context, db sqlz.DB, schemaName string) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtTables, schemaName, schemaName)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	var tables []*metadata.Table
	for rows.Next() {
		var tblSchema, tblName, tblType string
		var comment sql.NullString
		if err = rows.Scan(&tblSchema, &tblName, &tblType, &comment); err != nil {
			return nil, errz.Err(err)
		}

		tbl := &metadata.Table{
			Name:        tblName,
			FQName:      tblSchema + "." + tblName,
			DBTableType: tblType,
			Comment:     comment.String,
		}
		switch tblType {
		case "TABLE":
			tbl.TableType = sqlz.TableTypeTable
		case "VIEW":
			tbl.TableType = sqlz.TableTypeView
		}

		tables = append(tables, tbl)
	}
	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	// Fetch columns for each table. Row counts are obtained in batch below.
	for _, tbl := range tables {
		tbl.Columns, err = getColumnMetadata(ctx, db, schemaName, tbl.Name)
		if err != nil {
			return nil, err
		}
	}

	// Fetch row counts via a single UNION ALL query for efficiency.
	rowCounts, err := getTableRowCounts(ctx, db, tables)
	if err != nil {
		return nil, err
	}
	for i, tbl := range tables {
		tbl.RowCount = rowCounts[i]
	}

	return tables, nil
}

// getTableMetadata returns metadata for a single named table in db.
func getTableMetadata(ctx context.Context, db sqlz.DB, schemaName, tblName string) (*metadata.Table, error) {
	tbl := &metadata.Table{
		Name:   tblName,
		FQName: schemaName + "." + tblName,
	}

	// Determine table type and comment.
	const q = `SELECT 'TABLE' AS table_type, comment
FROM duckdb_tables()
WHERE NOT internal AND schema_name = ? AND table_name = ?
UNION ALL
SELECT 'VIEW', comment
FROM duckdb_views()
WHERE NOT internal AND schema_name = ? AND view_name = ?
LIMIT 1`

	var tblType string
	var comment sql.NullString
	if err := db.QueryRowContext(ctx, q, schemaName, tblName, schemaName, tblName).
		Scan(&tblType, &comment); err != nil {
		if err == sql.ErrNoRows {
			return nil, errz.Errorf("table not found: %s.%s", schemaName, tblName)
		}
		return nil, errz.Err(err)
	}

	tbl.DBTableType = tblType
	tbl.Comment = comment.String
	switch tblType {
	case "TABLE":
		tbl.TableType = sqlz.TableTypeTable
	case "VIEW":
		tbl.TableType = sqlz.TableTypeView
	}

	// Fetch row count.
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tblName)).
		Scan(&tbl.RowCount); err != nil {
		return nil, errz.Err(err)
	}

	var err error
	tbl.Columns, err = getColumnMetadata(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}

	return tbl, nil
}

// getColumnMetadata returns ordered column metadata for the given table.
func getColumnMetadata(ctx context.Context, db sqlz.DB, schemaName, tblName string) ([]*metadata.Column, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtColumns, schemaName, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	// Collect primary key column names for this table so we can mark them below.
	pkCols, err := getPKColumnNames(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}
	pkSet := make(map[string]bool, len(pkCols))
	for _, pk := range pkCols {
		pkSet[pk] = true
	}

	var cols []*metadata.Column
	for rows.Next() {
		var colName, dataType string
		var colIndex int64
		var colDefault, comment sql.NullString
		var isNullable bool

		if err = rows.Scan(&colName, &colIndex, &colDefault, &isNullable, &dataType, &comment); err != nil {
			return nil, errz.Err(err)
		}

		col := &metadata.Column{
			Name:         colName,
			Position:     colIndex,
			BaseType:     dataType,
			ColumnType:   dataType,
			Kind:         kindFromDBTypeName(dataType),
			Nullable:     isNullable,
			DefaultValue: colDefault.String,
			Comment:      comment.String,
			PrimaryKey:   pkSet[colName],
		}
		cols = append(cols, col)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return cols, nil
}

// getPKColumnNames returns the primary-key column names for the given table.
// Returns an empty slice when the table has no primary key.
func getPKColumnNames(ctx context.Context, db sqlz.DB, schemaName, tblName string) ([]string, error) {
	var raw string
	err := db.QueryRowContext(ctx, stmtPrimaryKeys, schemaName, tblName).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errz.Err(err)
	}

	// DuckDB returns the array as a string like "[actor_id]" when CAST to VARCHAR.
	// Strip the brackets and split on comma-space.
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ", ")
	return parts, nil
}

// getTableRowCounts returns row counts for the given tables using a single
// UNION ALL query. The returned slice is parallel to tables.
func getTableRowCounts(ctx context.Context, db sqlz.DB, tables []*metadata.Table) ([]int64, error) {
	log := lg.FromContext(ctx)

	if len(tables) == 0 {
		return nil, nil
	}

	var sb strings.Builder
	for i, tbl := range tables {
		if i > 0 {
			sb.WriteString(" UNION ALL ")
		}
		// Use COUNT(*) on each table. Views are also counted this way.
		fmt.Fprintf(&sb, "SELECT COUNT(*) FROM %q", tbl.Name)
	}

	rows, err := db.QueryContext(ctx, sb.String())
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	counts := make([]int64, len(tables))
	i := 0
	for rows.Next() {
		if err = rows.Scan(&counts[i]); err != nil {
			return nil, errz.Err(err)
		}
		i++
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return counts, nil
}

// listSchemas returns schema names visible in the current catalog.
func listSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtSchemaNames)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, errz.Err(err)
		}
		schemas = append(schemas, s)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return schemas, nil
}

// listSchemaMetadata returns *metadata.Schema values for all user-visible
// schemas in the current catalog.
func listSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtSchemas)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []*metadata.Schema
	for rows.Next() {
		var name, catalog, owner string
		if err = rows.Scan(&name, &catalog, &owner); err != nil {
			return nil, errz.Err(err)
		}
		schemas = append(schemas, &metadata.Schema{
			Name:    name,
			Catalog: catalog,
			Owner:   owner,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return schemas, nil
}

// schemaExists reports whether schemaName exists in the current catalog.
func schemaExists(ctx context.Context, db sqlz.DB, schemaName string) (bool, error) {
	if schemaName == "" {
		return false, nil
	}

	var count int
	err := db.QueryRowContext(ctx, stmtSchemaExists, schemaName).Scan(&count)
	if err != nil {
		return false, errz.Err(err)
	}

	return count > 0, nil
}

// buildTableNamesQuery constructs the SQL query for ListTableNames.
func buildTableNamesQuery(schemaName string, tables, views bool) string {
	if !tables && !views {
		return ""
	}

	schemaFilter := "current_schema()"
	if schemaName != "" {
		schemaFilter = fmt.Sprintf("'%s'", strings.ReplaceAll(schemaName, "'", "''"))
	}

	var parts []string
	if tables {
		parts = append(parts,
			`SELECT table_name FROM duckdb_tables() WHERE NOT internal AND schema_name = `+schemaFilter)
	}
	if views {
		parts = append(parts,
			`SELECT view_name FROM duckdb_views() WHERE NOT internal AND schema_name = `+schemaFilter)
	}

	return strings.Join(parts, " UNION ALL ") + " ORDER BY 1"
}

// listTableNames returns the names of tables and/or views in the given schema.
func listTableNames(ctx context.Context, db sqlz.DB, schemaName string, tables, views bool) ([]string, error) {
	log := lg.FromContext(ctx)

	if !tables && !views {
		return []string{}, nil
	}

	q := buildTableNamesQuery(schemaName, tables, views)

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer sqlz.CloseRows(log, rows)

	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errz.Err(err)
		}
		names = append(names, name)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return names, nil
}

// kindFromDBTypeName maps a DuckDB SQL type name to a kind.Kind.
// Composite types (LIST/STRUCT/MAP) all map to kind.Text; they are projected
// as JSON via to_json() at scan time (see RecordMeta in Task 4.3).
//
// DuckDB type names returned by duckdb_columns() include parametric forms
// like "DECIMAL(18,4)" and composite forms like "INTEGER[]" or
// "STRUCT(a INTEGER, b VARCHAR)". This function strips parameters and
// detects composites before doing the scalar lookup.
func kindFromDBTypeName(name string) kind.Kind {
	upper := strings.ToUpper(strings.TrimSpace(name))

	// Composites: detected by suffix-bracket or known prefixes.
	if strings.HasSuffix(upper, "[]") ||
		strings.HasPrefix(upper, "STRUCT") ||
		strings.HasPrefix(upper, "MAP") ||
		strings.HasPrefix(upper, "ENUM") {
		return kind.Text
	}

	// Strip parameter parens (e.g. "DECIMAL(18,4)" -> "DECIMAL").
	if i := strings.Index(upper, "("); i > 0 {
		upper = strings.TrimSpace(upper[:i])
	}

	switch upper {
	case "BOOLEAN", "BOOL":
		return kind.Bool
	case "TINYINT", "SMALLINT", "INTEGER", "INT", "INT4", "BIGINT", "INT8",
		"HUGEINT", "INT128", "UTINYINT", "USMALLINT", "UINTEGER", "UBIGINT", "UHUGEINT":
		return kind.Int
	case "FLOAT", "REAL", "FLOAT4", "DOUBLE", "FLOAT8":
		return kind.Float
	case "DECIMAL", "NUMERIC":
		return kind.Decimal
	case "VARCHAR", "CHAR", "TEXT", "STRING", "BPCHAR", "JSON", "UUID", "INTERVAL", "BIT":
		return kind.Text
	case "BLOB", "BYTEA", "BINARY", "VARBINARY":
		return kind.Bytes
	case "DATE":
		return kind.Date
	case "TIME", "TIME WITH TIME ZONE", "TIMETZ":
		return kind.Time
	case "TIMESTAMP", "TIMESTAMP_S", "TIMESTAMP_MS", "TIMESTAMP_NS",
		"TIMESTAMP WITH TIME ZONE", "TIMESTAMPTZ", "DATETIME":
		return kind.Datetime
	default:
		return kind.Unknown
	}
}

// tableExists reports whether a table or view named tblName exists
// in the current schema.
func tableExists(ctx context.Context, db sqlz.DB, tblName string) (bool, error) {
	const q = `SELECT COUNT(*) FROM (
SELECT table_name FROM duckdb_tables() WHERE schema_name = current_schema() AND table_name = ?
UNION ALL
SELECT view_name FROM duckdb_views() WHERE schema_name = current_schema() AND view_name = ?
) t`

	var count int
	err := db.QueryRowContext(ctx, q, tblName, tblName).Scan(&count)
	if err != nil {
		return false, errz.Err(err)
	}

	return count > 0, nil
}
