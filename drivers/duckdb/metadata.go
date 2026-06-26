package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
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

	// stmtSchemaNames lists only schema names in the current catalog.
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

	// stmtTables lists user tables and views in a given schema. DuckDB
	// exposes tables and views in separate catalog functions (duckdb_tables
	// and duckdb_views), so we UNION ALL them with hard-coded type labels.
	stmtTables = `SELECT schema_name, table_name, 'TABLE' AS table_type, comment
FROM duckdb_tables()
WHERE NOT internal AND schema_name = ?
UNION ALL
SELECT schema_name, view_name, 'VIEW', comment
FROM duckdb_views()
WHERE NOT internal AND schema_name = ?
ORDER BY table_name`

	// stmtColumns lists columns for a given (schema, table), ordered by position.
	stmtColumns = `SELECT column_name, column_index, column_default, is_nullable, data_type, comment
FROM duckdb_columns()
WHERE schema_name = ? AND table_name = ?
ORDER BY column_index`

	// stmtPrimaryKeys returns the primary-key column names for a given table,
	// one name per row. UNNEST yields each name as a separate row so that
	// column names containing comma or space are returned intact.
	stmtPrimaryKeys = `SELECT UNNEST(constraint_column_names)
FROM duckdb_constraints()
WHERE schema_name = ? AND table_name = ? AND constraint_type = 'PRIMARY KEY'`

	// stmtForeignKeysAll returns one row per (constraint, column-pair) for
	// every FOREIGN KEY constraint declared in the given schema. The
	// range/index pattern preserves positional pairing between
	// constraint_column_names and referenced_column_names for composite keys.
	// Rows are ordered so that all rows of a single constraint are
	// contiguous and in column-position order.
	//
	// DuckDB exposes neither referential actions (ON DELETE / ON UPDATE)
	// nor cross-schema referenced qualifiers in duckdb_constraints(),
	// so OnDelete/OnUpdate are left empty and references are assumed
	// to target the same schema.
	stmtForeignKeysAll = `SELECT c.table_name, c.constraint_name, c.referenced_table,
       c.constraint_column_names[i+1] AS local_col,
       c.referenced_column_names[i+1] AS ref_col
FROM duckdb_constraints() c,
     range(0, CAST(length(c.constraint_column_names) AS BIGINT)) AS r(i)
WHERE c.constraint_type = 'FOREIGN KEY'
  AND c.database_name = current_database()
  AND c.schema_name = ?
ORDER BY c.table_name, c.constraint_index, i`

	// stmtForeignKeysTable is the single-table form of stmtForeignKeysAll —
	// outgoing FK constraints declared on tblName.
	stmtForeignKeysTable = `SELECT c.constraint_name, c.referenced_table,
       c.constraint_column_names[i+1] AS local_col,
       c.referenced_column_names[i+1] AS ref_col
FROM duckdb_constraints() c,
     range(0, CAST(length(c.constraint_column_names) AS BIGINT)) AS r(i)
WHERE c.constraint_type = 'FOREIGN KEY'
  AND c.database_name = current_database()
  AND c.schema_name = ?
  AND c.table_name = ?
ORDER BY c.constraint_index, i`

	// stmtIncomingFKsTable returns the foreign-key constraints declared on
	// other tables whose referenced side is tblName. The owning (referencing)
	// table is reported via c.table_name.
	stmtIncomingFKsTable = `SELECT c.table_name, c.constraint_name,
       c.constraint_column_names[i+1] AS local_col,
       c.referenced_column_names[i+1] AS ref_col
FROM duckdb_constraints() c,
     range(0, CAST(length(c.constraint_column_names) AS BIGINT)) AS r(i)
WHERE c.constraint_type = 'FOREIGN KEY'
  AND c.database_name = current_database()
  AND c.schema_name = ?
  AND c.referenced_table = ?
ORDER BY c.table_name, c.constraint_index, i`

	// stmtUniqueConstraintsAll returns UNIQUE-constraint declarations for the
	// given schema. Primary keys are reported via Column.PrimaryKey and are
	// deliberately excluded here. constraint_column_names is unnested to one
	// row per column so composite UNIQUE constraints reassemble cleanly.
	stmtUniqueConstraintsAll = `SELECT c.table_name, c.constraint_name,
       c.constraint_column_names[i+1] AS col
FROM duckdb_constraints() c,
     range(0, CAST(length(c.constraint_column_names) AS BIGINT)) AS r(i)
WHERE c.constraint_type = 'UNIQUE'
  AND c.database_name = current_database()
  AND c.schema_name = ?
ORDER BY c.table_name, c.constraint_index, i`

	// stmtUniqueConstraintsTable is the single-table form of
	// stmtUniqueConstraintsAll.
	stmtUniqueConstraintsTable = `SELECT c.constraint_name,
       c.constraint_column_names[i+1] AS col
FROM duckdb_constraints() c,
     range(0, CAST(length(c.constraint_column_names) AS BIGINT)) AS r(i)
WHERE c.constraint_type = 'UNIQUE'
  AND c.database_name = current_database()
  AND c.schema_name = ?
  AND c.table_name = ?
ORDER BY c.constraint_index, i`

	// stmtIndexesAll returns one row per user-visible index in the given
	// schema. duckdb_indexes() reports only explicit CREATE INDEX
	// definitions — PK-backing and UNIQUE-constraint-backing indexes are
	// not surfaced here, so Table.Indexes for DuckDB contains only
	// manually-declared indexes. PK info is still available via
	// Column.PrimaryKey and UNIQUE info via Table.UniqueConstraints.
	//
	// duckdb_indexes().expressions is a VARCHAR holding the stringified
	// list of index-key expressions (e.g. "[name]", "[store_id, film_id]",
	// "['(lower(email))']"). The list is parsed Go-side by
	// parseDuckDBIndexExpressions so we can filter out functional keys
	// without depending on DuckDB string-parsing functions.
	stmtIndexesAll = `SELECT table_name, index_name, is_unique, expressions
FROM duckdb_indexes()
WHERE database_name = current_database()
  AND schema_name = ?
ORDER BY table_name, index_name`

	// stmtIndexesTable is the single-table form of stmtIndexesAll.
	stmtIndexesTable = `SELECT index_name, is_unique, expressions
FROM duckdb_indexes()
WHERE database_name = current_database()
  AND schema_name = ?
  AND table_name = ?
ORDER BY index_name`

	// stmtCheckConstraintsAll returns every CHECK constraint declared in the
	// given schema. Rows are ordered so constraints on the same table are
	// contiguous and in declaration order.
	stmtCheckConstraintsAll = `SELECT table_name, constraint_name, constraint_text
FROM duckdb_constraints()
WHERE database_name = current_database()
  AND schema_name = ?
  AND constraint_type = 'CHECK'
ORDER BY table_name, constraint_index`

	// stmtCheckConstraintsTable is the single-table form of stmtCheckConstraintsAll.
	stmtCheckConstraintsTable = `SELECT constraint_name, constraint_text
FROM duckdb_constraints()
WHERE database_name = current_database()
  AND schema_name = ?
  AND table_name = ?
  AND constraint_type = 'CHECK'
ORDER BY constraint_index`

	// stmtViewDefinitionsAll returns the sql DDL for every non-internal view
	// in the given schema.
	stmtViewDefinitionsAll = `SELECT view_name, sql
FROM duckdb_views()
WHERE database_name = current_database()
  AND schema_name = ?
  AND NOT internal
ORDER BY view_name`

	// stmtViewDefinitionTable returns the sql DDL for a single named view.
	stmtViewDefinitionTable = `SELECT sql
FROM duckdb_views()
WHERE database_name = current_database()
  AND schema_name = ?
  AND view_name = ?
  AND NOT internal`

	// stmtTableDDLAll returns the CREATE TABLE DDL for every non-internal
	// table in the given schema. Used to detect GENERATED ALWAYS AS columns,
	// which duckdb_columns() does not flag separately.
	stmtTableDDLAll = `SELECT table_name, sql
FROM duckdb_tables()
WHERE database_name = current_database()
  AND schema_name = ?
  AND NOT internal
ORDER BY table_name`

	// stmtTableDDLTable is the single-table form of stmtTableDDLAll.
	stmtTableDDLTable = `SELECT sql
FROM duckdb_tables()
WHERE database_name = current_database()
  AND schema_name = ?
  AND table_name = ?
  AND NOT internal`
)

// reIdxBareCol matches a bare unquoted SQL identifier as it appears in
// duckdb_indexes().expressions, e.g. `email` in `[email]`.
var reIdxBareCol = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reIdxQuotedCol matches a single-quote-wrapped double-quoted identifier,
// the form DuckDB emits in expressions when the column was originally
// quoted in the DDL or collides with a reserved word, e.g. `'"name"'`. An
// embedded double-quote in the column name is escaped by doubling it, so a
// column named a"b is emitted as `'"a""b"'`; the inner pattern allows those
// doubled quotes. The captured group is the (still-escaped) identifier;
// callers must un-double the quotes to recover the original name.
var reIdxQuotedCol = regexp.MustCompile(`^'"((?:[^"]|"")*)"'$`)

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
		return nil, errw(err)
	}
	md.DBVersion = strings.TrimPrefix(ver, "v")
	md.DBProduct = "DuckDB " + md.DBVersion

	// Fetch current catalog and schema.
	if err := db.QueryRowContext(ctx, stmtCurrentCatalog).Scan(&md.Name); err != nil {
		return nil, errw(err)
	}
	md.Catalog = md.Name

	var schema string
	if err := db.QueryRowContext(ctx, stmtCurrentSchema).Scan(&schema); err != nil {
		return nil, errw(err)
	}
	md.Schema = schema
	md.FQName = md.Name + "." + schema

	if fp := filePathFromLocation(src.Location); fp != "" {
		fi, err := os.Stat(fp)
		if err != nil {
			return nil, errz.Wrapf(err, "stat duckdb file for %s", src.Handle)
		}
		size := fi.Size()
		md.Size = &size
	}

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
	md.RecomputeTableCounts()

	// Populate FK / unique-constraint / check-constraint / index metadata at
	// the schema level in batched queries. LinkForeignKeys derives incoming
	// edges across tables as the final step.
	log := lg.FromContext(ctx)
	fks, err := getSchemaForeignKeys(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	metadata.AssignForeignKeys(log, md.Tables, fks)

	ucs, err := getSchemaUniqueConstraints(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	metadata.AssignUniqueConstraints(log, md.Tables, ucs)

	checks, err := getSchemaCheckConstraints(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	metadata.AssignCheckConstraints(log, md.Tables, checks)

	idxs, err := getSchemaIndexes(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	metadata.AssignIndexes(log, md.Tables, idxs)

	// Populate ViewDefinition for each view.
	viewDefs, err := getSchemaViewDefinitions(ctx, db, schema)
	if err != nil {
		return nil, err
	}
	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeView {
			tbl.ViewDefinition = viewDefs[tbl.Name]
		}
	}

	metadata.LinkForeignKeys(log, md)

	return md, nil
}

// getSchemaTableMetadata returns metadata for every table/view in schemaName.
func getSchemaTableMetadata(ctx context.Context, db sqlz.DB, schemaName string) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtTables, schemaName, schemaName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var tables []*metadata.Table
	for rows.Next() {
		var tblSchema, tblName, tblType string
		var comment sql.NullString
		if err = rows.Scan(&tblSchema, &tblName, &tblType, &comment); err != nil {
			return nil, errw(err)
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
		return nil, errw(err)
	}

	// Fetch generated-column info for all tables in the schema in one query.
	// duckdb_columns() doesn't flag generated columns separately, so we detect
	// them by parsing the canonical DDL from duckdb_tables().sql.
	generatedCols, err := getSchemaGeneratedColumns(ctx, db, schemaName)
	if err != nil {
		return nil, err
	}

	// Fetch columns for each table. Row counts are obtained in batch below.
	for _, tbl := range tables {
		tbl.Columns, err = getColumnMetadata(ctx, db, schemaName, tbl.Name)
		if err != nil {
			return nil, err
		}
		applyGeneratedColumns(tbl.Columns, generatedCols[tbl.Name])
	}

	rowCounts, err := getTableRowCounts(ctx, db, schemaName, tables)
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errz.Errorf("table not found: %s.%s", schemaName, tblName)
		}
		return nil, errw(err)
	}

	tbl.DBTableType = tblType
	tbl.Comment = comment.String
	switch tblType {
	case "TABLE":
		tbl.TableType = sqlz.TableTypeTable
	case "VIEW":
		tbl.TableType = sqlz.TableTypeView
	}

	// Fetch row count. Schema-qualify the table reference so the count is
	// correct even if the connection's current schema differs from schemaName.
	if err := db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM %q.%q`, schemaName, tblName)).
		Scan(&tbl.RowCount); err != nil {
		return nil, errw(err)
	}

	var err error
	tbl.Columns, err = getColumnMetadata(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}

	// Load view definition for views; constraint/index introspection is
	// table-only.
	if tbl.TableType == sqlz.TableTypeView {
		tbl.ViewDefinition, err = getTableViewDefinition(ctx, db, schemaName, tblName)
		if err != nil {
			return nil, err
		}
		return tbl, nil
	}

	// Tables only — duckdb_constraints() / duckdb_indexes() don't apply
	// to views and would return no rows anyway, so skip the round-trips.
	if tbl.TableType != sqlz.TableTypeTable {
		return tbl, nil
	}

	// Detect generated columns by parsing the CREATE TABLE DDL.
	// duckdb_columns() does not expose an is_generated flag.
	genCols, err := getTableGeneratedColumnNames(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}
	applyGeneratedColumns(tbl.Columns, genCols)

	outgoing, err := getTableForeignKeys(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}
	incoming, err := getTableIncomingFKs(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}
	tbl.FK = metadata.NewFKGroup(outgoing, incoming)

	tbl.UniqueConstraints, err = getTableUniqueConstraints(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}

	tbl.CheckConstraints, err = getTableCheckConstraints(ctx, db, schemaName, tblName)
	if err != nil {
		return nil, err
	}

	tbl.Indexes, err = getTableIndexes(ctx, db, schemaName, tblName)
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
		return nil, errw(err)
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
			return nil, errw(err)
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
		return nil, errw(err)
	}

	return cols, nil
}

// getPKColumnNames returns the primary-key column names for the given table.
// Returns an empty slice when the table has no primary key. UNNEST yields
// each name as a separate row so column names containing comma or space are
// preserved.
func getPKColumnNames(ctx context.Context, db sqlz.DB, schemaName, tblName string) ([]string, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtPrimaryKeys, schemaName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var cols []string
	for rows.Next() {
		var col string
		if err = rows.Scan(&col); err != nil {
			return nil, errw(err)
		}
		cols = append(cols, col)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return cols, nil
}

// getSchemaForeignKeys returns every FOREIGN KEY constraint declared in
// schemaName, as outgoing FKs (one [metadata.ForeignKey] per constraint
// with positional Columns/RefColumns for composite keys). Cross-table
// linking ([FKGroup.Incoming]) is left to [metadata.LinkForeignKeys].
func getSchemaForeignKeys(ctx context.Context, db sqlz.DB, schemaName string) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtForeignKeysAll, schemaName)
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
		var tblName, fkName, refTable, localCol, refCol string
		if err = rows.Scan(&tblName, &fkName, &refTable, &localCol, &refCol); err != nil {
			return nil, errw(err)
		}
		k := fkKey{table: tblName, name: fkName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:     fkName,
				Table:    tblName,
				RefTable: refTable,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, localCol)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	return fks, errw(rows.Err())
}

// getTableForeignKeys returns outgoing FK constraints declared on
// (schemaName, tblName). Per-table analog of [getSchemaForeignKeys].
func getTableForeignKeys(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtForeignKeysTable, schemaName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	byName := map[string]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		var fkName, refTable, localCol, refCol string
		if err = rows.Scan(&fkName, &refTable, &localCol, &refCol); err != nil {
			return nil, errw(err)
		}
		fk, ok := byName[fkName]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:     fkName,
				Table:    tblName,
				RefTable: refTable,
			}
			byName[fkName] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, localCol)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	return fks, errw(rows.Err())
}

// getTableIncomingFKs returns the FK constraints declared on other
// tables in schemaName whose referenced side is tblName.
func getTableIncomingFKs(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtIncomingFKsTable, schemaName, tblName)
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
		var owningTbl, fkName, localCol, refCol string
		if err = rows.Scan(&owningTbl, &fkName, &localCol, &refCol); err != nil {
			return nil, errw(err)
		}
		k := fkKey{table: owningTbl, name: fkName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:     fkName,
				Table:    owningTbl,
				RefTable: tblName,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, localCol)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	return fks, errw(rows.Err())
}

// getSchemaUniqueConstraints returns every UNIQUE constraint declared in
// schemaName, with positional Columns for composite constraints.
func getSchemaUniqueConstraints(ctx context.Context, db sqlz.DB, schemaName string,
) ([]*metadata.UniqueConstraint, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtUniqueConstraintsAll, schemaName)
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
		var tblName, ucName, col string
		if err = rows.Scan(&tblName, &ucName, &col); err != nil {
			return nil, errw(err)
		}
		k := ucKey{table: tblName, name: ucName}
		uc, ok := byKey[k]
		if !ok {
			uc = &metadata.UniqueConstraint{Name: ucName, Table: tblName}
			byKey[k] = uc
			ucs = append(ucs, uc)
		}
		uc.Columns = append(uc.Columns, col)
	}
	return ucs, errw(rows.Err())
}

// getTableUniqueConstraints is the per-table analog of
// [getSchemaUniqueConstraints].
func getTableUniqueConstraints(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) ([]*metadata.UniqueConstraint, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtUniqueConstraintsTable, schemaName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	byName := map[string]*metadata.UniqueConstraint{}
	var ucs []*metadata.UniqueConstraint
	for rows.Next() {
		var ucName, col string
		if err = rows.Scan(&ucName, &col); err != nil {
			return nil, errw(err)
		}
		uc, ok := byName[ucName]
		if !ok {
			uc = &metadata.UniqueConstraint{Name: ucName, Table: tblName}
			byName[ucName] = uc
			ucs = append(ucs, uc)
		}
		uc.Columns = append(uc.Columns, col)
	}
	return ucs, errw(rows.Err())
}

// getSchemaIndexes returns every user-declared index in schemaName.
// Index keys that aren't plain column identifiers (functional indexes
// like `CREATE INDEX ix ON t(LOWER(c))`) are recorded as the empty-string
// sentinel in [metadata.Index.Columns]; indexes whose every key is an
// expression are omitted entirely.
func getSchemaIndexes(ctx context.Context, db sqlz.DB, schemaName string,
) ([]*metadata.Index, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtIndexesAll, schemaName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var idxs []*metadata.Index
	for rows.Next() {
		var tblName, idxName, exprList string
		var isUnique bool
		if err = rows.Scan(&tblName, &idxName, &isUnique, &exprList); err != nil {
			return nil, errw(err)
		}
		cols := parseDuckDBIndexExpressions(exprList)
		if len(cols) == 0 || metadata.AllExpressionKeys(cols) {
			logDroppedDuckDBIndex(log, tblName, idxName, exprList)
			continue
		}
		idxs = append(idxs, &metadata.Index{
			Name:    idxName,
			Table:   tblName,
			Columns: cols,
			Unique:  isUnique,
		})
	}
	return idxs, errw(rows.Err())
}

// getTableIndexes is the per-table analog of [getSchemaIndexes].
func getTableIndexes(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) ([]*metadata.Index, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtIndexesTable, schemaName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var idxs []*metadata.Index
	for rows.Next() {
		var idxName, exprList string
		var isUnique bool
		if err = rows.Scan(&idxName, &isUnique, &exprList); err != nil {
			return nil, errw(err)
		}
		cols := parseDuckDBIndexExpressions(exprList)
		if len(cols) == 0 || metadata.AllExpressionKeys(cols) {
			logDroppedDuckDBIndex(log, tblName, idxName, exprList)
			continue
		}
		idxs = append(idxs, &metadata.Index{
			Name:    idxName,
			Table:   tblName,
			Columns: cols,
			Unique:  isUnique,
		})
	}
	return idxs, errw(rows.Err())
}

// logDroppedDuckDBIndex emits a log entry for an index dropped from
// the metadata because [parseDuckDBIndexExpressions] returned no
// usable plain-column entries. Three cases are distinguished:
//
//   - Empty list ("[]" / "[ ]"): an index with no key columns. Debug
//     level — uncommon but well-formed DuckDB output.
//   - Well-formed list with no plain-column tokens (e.g.
//     "['(lower(email))']"): a functional / expression-only index.
//     Debug level — this is legitimate DuckDB usage; the index is
//     dropped from [metadata.Table.Indexes] because every key is an
//     expression (see [metadata.AllExpressionKeys]).
//   - Anything else (no surrounding brackets, garbled output): a
//     genuine surprise — likely a DuckDB output-format change.
//     Warn level so it's visible in operator logs.
func logDroppedDuckDBIndex(log *slog.Logger, tblName, idxName, exprList string) {
	if log == nil {
		return
	}
	attrs := []any{
		slog.String("table", tblName),
		slog.String("index", idxName),
		slog.String("expressions", exprList),
	}
	trimmed := strings.TrimSpace(exprList)
	// Well-formed `[...]` shape — either empty or all-functional;
	// either case is legitimate DuckDB output, not a format change.
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		log.Debug("duckdb: dropping index with no plain-column keys", attrs...)
		return
	}
	log.Warn("duckdb: dropping index whose expressions string is in an unrecognized format", attrs...)
}

// parseDuckDBIndexExpressions parses the stringified list returned in
// duckdb_indexes().expressions and returns the keys in declaration order.
// Plain column identifiers are returned as-is. Functional / expression
// keys are recorded as the empty-string sentinel, matching the contract
// on [metadata.Index.Columns].
//
// The DuckDB output format is a bracket-wrapped, comma-separated list:
//
//	"[last_name]"                       (single column)
//	"[store_id, film_id]"               (composite)
//	"['(lower(email))']"                (functional — sentinel "")
//	"[name, '(lower(email))']"          (mixed — `name`, then sentinel "")
//
// Column names that themselves contain a comma or space would round-trip
// as double-quoted identifiers in the list (e.g. `["first, last"]`).
// Splitting at top-level commas (i.e. commas not inside `'` or `"`) keeps
// those intact.
func parseDuckDBIndexExpressions(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		return nil
	}

	var (
		parts         []string
		buf           strings.Builder
		inSingle      bool
		inDouble      bool
		prevBackslash bool
	)
	flush := func() {
		parts = append(parts, strings.TrimSpace(buf.String()))
		buf.Reset()
	}
	for _, r := range inner {
		switch {
		case prevBackslash:
			buf.WriteRune(r)
			prevBackslash = false
		case r == '\\':
			buf.WriteRune(r)
			prevBackslash = true
		case r == '\'' && !inDouble:
			buf.WriteRune(r)
			inSingle = !inSingle
		case r == '"' && !inSingle:
			buf.WriteRune(r)
			inDouble = !inDouble
		case r == ',' && !inSingle && !inDouble:
			flush()
		default:
			buf.WriteRune(r)
		}
	}
	flush()

	out := make([]string, 0, len(parts))
	for _, p := range parts {
		switch {
		case reIdxBareCol.MatchString(p):
			out = append(out, p)
		case reIdxQuotedCol.MatchString(p):
			// Un-double any escaped quotes to recover the real column
			// name (DuckDB emits a"b as `'"a""b"'`).
			name := strings.ReplaceAll(reIdxQuotedCol.FindStringSubmatch(p)[1], `""`, `"`)
			out = append(out, name)
		default:
			// Functional / expression key: record the empty-string
			// sentinel so Columns preserves the index's key arity.
			out = append(out, "")
		}
	}
	return out
}

// getTableRowCounts returns row counts for the given tables. The returned
// slice is parallel to tables. Each table is queried individually so that a
// race with concurrent DROP TABLE in another session (common during parallel
// test runs that share a fixture file) just yields a 0 count for that table
// rather than failing the whole metadata fetch.
//
// The fallback uses the typed *driver.NotExistError, so any catalog-level
// "does not exist" error that errw recognizes (Table, View, Schema) is
// coerced to a zero row count. A concurrent schema drop is therefore
// also swallowed, which is acceptable for the parallel-test scenario this
// guards. Errors not recognized by errw (e.g. column not found, syntax
// errors, connection failures) surface unchanged.
func getTableRowCounts(ctx context.Context, db sqlz.DB, schemaName string,
	tables []*metadata.Table,
) ([]int64, error) {
	log := lg.FromContext(ctx)
	counts := make([]int64, len(tables))
	for i, tbl := range tables {
		q := fmt.Sprintf(`SELECT COUNT(*) FROM %q.%q`, schemaName, tbl.Name)
		err := db.QueryRowContext(ctx, q).Scan(&counts[i])
		if err == nil {
			continue
		}
		// errw tags "Catalog Error: Table ... does not exist" as NotExistError.
		wrapped := errw(err)
		var nfe *driver.NotExistError
		if errors.Is(err, sql.ErrNoRows) || errors.As(wrapped, &nfe) {
			log.Debug("getTableRowCounts: table missing, treating count as 0",
				"table", tbl.FQName, "err", err)
			continue
		}
		return nil, wrapped
	}
	return counts, nil
}

// reDuckDBGeneratedAlways matches the GENERATED ALWAYS AS marker in a
// DuckDB CREATE TABLE DDL string (case-insensitive).
var reDuckDBGeneratedAlways = regexp.MustCompile(`(?i)\bGENERATED\s+ALWAYS\s+AS\b`)

// getSchemaCheckConstraints returns all CHECK constraints declared in
// schemaName, as one *metadata.CheckConstraint per constraint row.
func getSchemaCheckConstraints(ctx context.Context, db sqlz.DB, schemaName string,
) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtCheckConstraintsAll, schemaName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var checks []*metadata.CheckConstraint
	for rows.Next() {
		var tblName, name, clause string
		if err = rows.Scan(&tblName, &name, &clause); err != nil {
			return nil, errw(err)
		}
		checks = append(checks, &metadata.CheckConstraint{
			Name:   name,
			Table:  tblName,
			Clause: clause,
		})
	}
	return checks, errw(rows.Err())
}

// getTableCheckConstraints is the per-table analog of [getSchemaCheckConstraints].
func getTableCheckConstraints(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtCheckConstraintsTable, schemaName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var checks []*metadata.CheckConstraint
	for rows.Next() {
		var name, clause string
		if err = rows.Scan(&name, &clause); err != nil {
			return nil, errw(err)
		}
		checks = append(checks, &metadata.CheckConstraint{
			Name:   name,
			Table:  tblName,
			Clause: clause,
		})
	}
	return checks, errw(rows.Err())
}

// getSchemaViewDefinitions returns a map of view_name -> DDL for all
// non-internal views in schemaName.
func getSchemaViewDefinitions(ctx context.Context, db sqlz.DB, schemaName string,
) (map[string]string, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtViewDefinitionsAll, schemaName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	defs := make(map[string]string)
	for rows.Next() {
		var viewName string
		var sqlDef sql.NullString
		if err = rows.Scan(&viewName, &sqlDef); err != nil {
			return nil, errw(err)
		}
		defs[viewName] = sqlDef.String
	}
	return defs, errw(rows.Err())
}

// getTableViewDefinition returns the CREATE VIEW DDL for a single named view.
// Returns an empty string when the view is not found.
func getTableViewDefinition(ctx context.Context, db sqlz.DB, schemaName, viewName string,
) (string, error) {
	var sqlDef sql.NullString
	err := db.QueryRowContext(ctx, stmtViewDefinitionTable, schemaName, viewName).Scan(&sqlDef)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", errw(err)
	}
	return sqlDef.String, nil
}

// getSchemaGeneratedColumns returns a map of table_name -> (set of column
// names that are declared GENERATED ALWAYS AS) for all non-internal tables
// in schemaName. The detection is based on parsing the CREATE TABLE DDL
// returned by duckdb_tables().sql, because duckdb_columns() does not expose
// a separate is_generated flag.
func getSchemaGeneratedColumns(ctx context.Context, db sqlz.DB, schemaName string,
) (map[string]map[string]bool, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtTableDDLAll, schemaName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	result := make(map[string]map[string]bool)
	for rows.Next() {
		var tblName string
		var ddl sql.NullString
		if err = rows.Scan(&tblName, &ddl); err != nil {
			return nil, errw(err)
		}
		if ddl.Valid && ddl.String != "" {
			if genCols := parseDuckDBGeneratedColumnNames(ddl.String); len(genCols) > 0 {
				result[tblName] = genCols
			}
		}
	}
	return result, errw(rows.Err())
}

// getTableGeneratedColumnNames returns the set of column names (for tblName)
// that are declared GENERATED ALWAYS AS. Returns nil when none are found.
func getTableGeneratedColumnNames(ctx context.Context, db sqlz.DB, schemaName, tblName string,
) (map[string]bool, error) {
	var ddl sql.NullString
	err := db.QueryRowContext(ctx, stmtTableDDLTable, schemaName, tblName).Scan(&ddl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, errw(err)
	}
	if !ddl.Valid || ddl.String == "" {
		return nil, nil
	}
	return parseDuckDBGeneratedColumnNames(ddl.String), nil
}

// parseDuckDBGeneratedColumnNames parses a DuckDB CREATE TABLE DDL string and
// returns the set of column names declared as GENERATED ALWAYS AS.
//
// DuckDB's duckdb_columns() catalog does not expose a separate is_generated
// flag; the generated expression appears in column_default alongside ordinary
// DEFAULT values. The canonical DDL from duckdb_tables().sql is the only
// reliable source to distinguish the two. The DDL format produced by DuckDB
// is regular enough for this targeted parse.
func parseDuckDBGeneratedColumnNames(ddl string) map[string]bool {
	// Locate the outer ( ... ) that wraps the column/constraint list.
	start := strings.Index(ddl, "(")
	if start < 0 {
		return nil
	}
	// Scan for the matching close-paren, tracking string literals so that
	// an unbalanced paren inside a quoted value (e.g. DEFAULT ':)') does not
	// terminate the search early.  The logic mirrors duckdbSplitTopLevel.
	depth := 0
	end := -1
	inSingle := false
	inDouble := false
	for i := start; i < len(ddl); i++ {
		c := ddl[i]
		switch {
		case inSingle:
			if c == '\'' {
				// SQL escaped quote '' — skip the second quote.
				if i+1 < len(ddl) && ddl[i+1] == '\'' {
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil
	}

	// Split the column-list at top-level commas.
	defs := duckdbSplitTopLevel(ddl[start+1:end], ',')

	result := make(map[string]bool)
	for _, def := range defs {
		def = strings.TrimSpace(def)
		// Strip string-literal contents before the regex match so that a
		// DEFAULT value containing the text "GENERATED ALWAYS AS" (e.g.
		// DEFAULT 'GENERATED ALWAYS AS legacy') does not produce a false
		// positive.
		if !reDuckDBGeneratedAlways.MatchString(duckdbStripStringLiterals(def)) {
			continue
		}
		// Skip table-level constraint clauses (CHECK, PRIMARY KEY, etc.) which
		// would never contain a column-name as their first token.
		upper := strings.ToUpper(strings.TrimLeft(def, " \t"))
		switch {
		case strings.HasPrefix(upper, "CHECK"),
			strings.HasPrefix(upper, "CONSTRAINT"),
			strings.HasPrefix(upper, "PRIMARY"),
			strings.HasPrefix(upper, "FOREIGN"),
			strings.HasPrefix(upper, "UNIQUE"):
			continue
		}
		if colName := duckdbFirstIdentifier(def); colName != "" {
			result[colName] = true
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// duckdbSplitTopLevel splits s at each occurrence of sep that is at nesting
// depth 0 (i.e. not inside paired parentheses or single/double quotes).
func duckdbSplitTopLevel(s string, sep rune) []string {
	var (
		parts    []string
		buf      strings.Builder
		depth    int
		inSingle bool
		inDouble bool
	)
	for _, r := range s {
		switch {
		case inSingle:
			buf.WriteRune(r)
			if r == '\'' {
				inSingle = false
			}
		case inDouble:
			buf.WriteRune(r)
			if r == '"' {
				inDouble = false
			}
		case r == '\'':
			buf.WriteRune(r)
			inSingle = true
		case r == '"':
			buf.WriteRune(r)
			inDouble = true
		case r == '(':
			buf.WriteRune(r)
			depth++
		case r == ')':
			buf.WriteRune(r)
			depth--
		case r == sep && depth == 0:
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

// duckdbStripStringLiterals returns s with the contents of single-quoted SQL
// string literals replaced by space characters, so that subsequent text scans
// (e.g. a regex looking for GENERATED ALWAYS AS) cannot match keywords that
// appear only inside a literal value. The surrounding quote characters are
// preserved to keep the string length stable; only the interior bytes are
// blanked. Double-quoted identifiers are left untouched because their contents
// are SQL names, not data values.
func duckdbStripStringLiterals(s string) string {
	// Fast path: if there are no single quotes, nothing to do.
	if !strings.Contains(s, "'") {
		return s
	}
	buf := []byte(s)
	inSingle := false
	for i := 0; i < len(buf); i++ {
		c := buf[i]
		if inSingle {
			if c == '\'' {
				// Check for escaped quote ('').
				if i+1 < len(buf) && buf[i+1] == '\'' {
					buf[i] = ' '
					buf[i+1] = ' '
					i++ // skip the second quote too
				} else {
					inSingle = false
					// Leave the closing quote as-is.
				}
			} else {
				buf[i] = ' '
			}
		} else if c == '\'' {
			inSingle = true
			// Leave the opening quote as-is.
		}
	}
	return string(buf)
}

// duckdbFirstIdentifier returns the first SQL identifier (bare or
// double-quoted) from the beginning of s after leading whitespace.
// For a double-quoted identifier, embedded doubled-quotes ("") are unescaped
// to recover the original name.
func duckdbFirstIdentifier(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	if s[0] == '"' {
		// Double-quoted identifier; walk past opening quote.
		var buf strings.Builder
		for i := 1; i < len(s); i++ {
			if s[i] == '"' {
				if i+1 < len(s) && s[i+1] == '"' {
					// Escaped inner double-quote.
					buf.WriteByte('"')
					i++
				} else {
					// Closing quote.
					break
				}
			} else {
				buf.WriteByte(s[i])
			}
		}
		return buf.String()
	}
	// Bare identifier: take until the first non-identifier character.
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return s[:i]
		}
	}
	return s
}

// applyGeneratedColumns marks columns in cols as generated if their name
// appears in genSet. The DefaultValue is moved to GeneratedExpr and cleared.
func applyGeneratedColumns(cols []*metadata.Column, genSet map[string]bool) {
	if len(genSet) == 0 {
		return
	}
	for _, col := range cols {
		if genSet[col.Name] {
			col.Generated = true
			col.GeneratedExpr = col.DefaultValue
			col.DefaultValue = ""
		}
	}
}

// listSchemas returns schema names visible in the current catalog.
func listSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtSchemaNames)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, s)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return schemas, nil
}

// listSchemaMetadata returns *metadata.Schema values for all user-visible
// schemas in the current catalog.
func listSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, stmtSchemas)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []*metadata.Schema
	for rows.Next() {
		var name, catalog, owner string
		if err = rows.Scan(&name, &catalog, &owner); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, &metadata.Schema{
			Name:    name,
			Catalog: catalog,
			Owner:   owner,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
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
		return false, errw(err)
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
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}
		names = append(names, name)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return names, nil
}

// kindFromDBTypeName maps a DuckDB SQL type name to a kind.Kind.
// Composite types (LIST/ARRAY rendered as "T[]", STRUCT, MAP, ENUM) all
// map to kind.Text; native go-duckdb composite values are stringified in
// newRecordFuncForDuckDB.
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
		"UTINYINT", "USMALLINT", "UINTEGER":
		return kind.Int
	case "HUGEINT", "INT128", "UBIGINT", "UHUGEINT":
		// These types can exceed int64 range:
		// - UBIGINT  max = 2^64 - 1 ≈ 1.8e19
		// - HUGEINT  max = 2^127 - 1 ≈ 1.7e38
		// - UHUGEINT max = 2^128 - 1 ≈ 3.4e38
		// Promote to kind.Decimal so values round-trip losslessly via
		// decimal.Decimal rather than truncating to int64.
		return kind.Decimal
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
		return false, errw(err)
	}

	return count > 0, nil
}
