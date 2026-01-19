package clickhouse

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the ClickHouse source, including
// database version, current database name, and optionally table/column metadata.
//
// Parameters:
//   - src: The source configuration
//   - db: The database connection
//   - noSchema: If true, only returns basic metadata without table details
//
// The returned metadata includes:
//   - Handle: The source handle (e.g., "@mydb")
//   - Location: The connection string
//   - Driver: drivertype.ClickHouse
//   - DBVersion: ClickHouse server version from version()
//   - Schema/Name: Current database from currentDatabase()
//   - Tables: Table metadata (if noSchema is false)
func getSourceMetadata(ctx context.Context, src *source.Source, db *sql.DB, noSchema bool) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:    src.Handle,
		Location:  src.Location,
		Driver:    drivertype.ClickHouse,
		DBVersion: "",
	}

	// Get database version
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		// If we can't query version, just leave it empty
		md.DBVersion = ""
	} else {
		md.DBVersion = version
	}

	// Get current database
	var database string
	err = db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&database)
	if err != nil {
		return nil, errw(err)
	}
	md.Schema = database
	md.Name = database

	if noSchema {
		// Don't fetch table metadata
		return md, nil
	}

	// Get table metadata
	tables, err := getTablesMetadata(ctx, db, database)
	if err != nil {
		return nil, err
	}

	md.Tables = tables
	return md, nil
}

// getTablesMetadata returns metadata for all tables and views in the specified
// database by querying the system.tables catalog table.
//
// For each table/view, it retrieves:
//   - name: Table name
//   - engine: ClickHouse engine type (MergeTree, View, MaterializedView, etc.)
//   - total_rows: Row count (may be approximate for some engines)
//   - total_bytes: Storage size in bytes
//
// Views (engine "View" or "MaterializedView") are included with TableType set
// to sqlz.TableTypeView. All other engines are considered tables.
//
// If column metadata retrieval fails for a table, a warning is logged and the
// table is skipped (not included in the result).
func getTablesMetadata(ctx context.Context, db *sql.DB, dbName string) ([]*metadata.Table, error) {
	const query = `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes
		FROM system.tables
		WHERE database = ?
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var tables []*metadata.Table
	for rows.Next() {
		var tblName, engine string
		var totalRows, totalBytes sql.NullInt64

		if err = rows.Scan(&tblName, &engine, &totalRows, &totalBytes); err != nil {
			return nil, errw(err)
		}

		tblMeta := &metadata.Table{
			Name:        tblName,
			DBTableType: engine,
			TableType:   tableTypeFromEngine(engine),
			RowCount:    totalRows.Int64,
		}

		if totalBytes.Valid {
			bytes := totalBytes.Int64
			tblMeta.Size = &bytes
		}

		// Get column metadata for this table
		cols, colErr := getColumnsMetadata(ctx, db, dbName, tblName)
		if colErr != nil {
			lg.FromContext(ctx).Warn("Failed to get column metadata for table, skipping",
				lga.Table, tblName, lga.DB, dbName, lga.Err, colErr)
			continue
		}
		tblMeta.Columns = cols

		tables = append(tables, tblMeta)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return tables, nil
}

// getTableMetadata returns metadata for a specific table, including its
// columns, by querying system.tables and system.columns.
//
// Parameters:
//   - dbName: Database name. If empty, uses currentDatabase().
//   - tblName: Table name to retrieve metadata for.
//
// Returns an error if the table does not exist or cannot be queried.
func getTableMetadata(ctx context.Context, db *sql.DB, dbName, tblName string) (*metadata.Table, error) {
	// If dbName is empty, use currentDatabase().
	if dbName == "" {
		err := db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&dbName)
		if err != nil {
			return nil, errw(err)
		}
	}

	const queryTable = `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes
		FROM system.tables
		WHERE database = ? AND name = ?
	`

	var tableName, engine string
	var totalRows, totalBytes sql.NullInt64

	err := db.QueryRowContext(ctx, queryTable, dbName, tblName).Scan(
		&tableName, &engine, &totalRows, &totalBytes)
	if err != nil {
		return nil, errw(err)
	}

	tblMeta := &metadata.Table{
		Name:        tblName,
		DBTableType: engine,
		TableType:   tableTypeFromEngine(engine),
		RowCount:    totalRows.Int64,
	}

	if totalBytes.Valid {
		bytes := totalBytes.Int64
		tblMeta.Size = &bytes
	}

	// Get column metadata
	cols, err := getColumnsMetadata(ctx, db, dbName, tblName)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	return tblMeta, nil
}

// getColumnsMetadata returns metadata for all columns in a table by querying
// the system.columns catalog table.
//
// For each column, it retrieves:
//   - name: Column name
//   - type: ClickHouse type string (e.g., "String", "Nullable(Int64)")
//   - position: Column ordinal position (1-based in ClickHouse)
//   - default_kind: Type of default (e.g., "DEFAULT", "MATERIALIZED")
//   - default_expression: Default value expression
//   - comment: Column comment
//
// The Kind field is derived from the ClickHouse type using kindFromClickHouseType.
// The Nullable field is determined using isNullableType, which checks if the
// outermost type wrapper is Nullable.
//
// Note: ClickHouse doesn't have traditional primary keys. The PrimaryKey field
// is always set to false. The ORDER BY clause in MergeTree tables defines
// sort order but not uniqueness constraints.
func getColumnsMetadata(ctx context.Context, db *sql.DB, dbName, tblName string) ([]*metadata.Column, error) {
	const query = `
		SELECT
			name,
			type,
			position,
			default_kind,
			default_expression,
			comment
		FROM system.columns
		WHERE database = ? AND table = ?
		ORDER BY position
	`

	rows, err := db.QueryContext(ctx, query, dbName, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var cols []*metadata.Column
	for rows.Next() {
		var colName, colType string
		var position int
		var defaultKind, defaultExpression, comment sql.NullString

		err = rows.Scan(&colName, &colType, &position, &defaultKind, &defaultExpression, &comment)
		if err != nil {
			return nil, errw(err)
		}

		col := &metadata.Column{
			Name:       colName,
			Position:   int64(position),
			Kind:       kindFromClickHouseType(colType),
			ColumnType: colType,
			// Use isNullableType (not isNullableTypeUnwrapped) because system.columns
			// reports the declared type directly. If a column is declared as
			// LowCardinality(Nullable(T)), that's what we get, and the column metadata
			// should reflect the declared nullability, not the unwrapped form.
			Nullable: isNullableType(colType),
			Comment:  comment.String,
		}

		// ClickHouse doesn't have traditional primary keys in the SQL sense
		// The ORDER BY clause defines sort order, not uniqueness
		col.PrimaryKey = false

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// tableTypeFromEngine returns the canonical table type (sqlz.TableTypeTable or
// sqlz.TableTypeView) based on the ClickHouse engine type. Views have engine
// "View" or "MaterializedView"; all other engines are considered tables.
func tableTypeFromEngine(engine string) string {
	switch engine {
	case "View", "MaterializedView":
		return sqlz.TableTypeView
	default:
		return sqlz.TableTypeTable
	}
}

// isNullableType checks if a ClickHouse type string has Nullable as its
// outermost wrapper. It returns true for "Nullable(String)" but false for
// "LowCardinality(Nullable(String))" because the outer wrapper is LowCardinality.
//
// Use this function when reading from system.columns, where the type string
// reflects the declared schema type. In system.columns, a column declared as
// "Nullable(String)" will have exactly that type string, making direct prefix
// matching correct.
//
// For query result processing where the ClickHouse driver may report types like
// "LowCardinality(Nullable(String))", use isNullableTypeUnwrapped instead.
func isNullableType(typeName string) bool {
	return len(typeName) > 9 && typeName[:9] == "Nullable("
}

// isNullableTypeUnwrapped checks if a ClickHouse type is nullable after
// stripping any LowCardinality wrapper. It returns true for both "Nullable(T)"
// and "LowCardinality(Nullable(T))".
//
// ClickHouse's LowCardinality is a storage optimization that can wrap Nullable
// types. When processing query results via sql.ColumnType.DatabaseTypeName(),
// the driver reports the full type including LowCardinality. For example, a
// column declared as "LowCardinality(Nullable(String))" will be reported with
// that exact type string.
//
// The standard sql.ColumnType.Nullable() method may not correctly report
// nullability for ClickHouse types, so we must detect it from the type name.
// This function handles both patterns:
//   - "Nullable(String)" -> true
//   - "LowCardinality(Nullable(String))" -> true
//   - "LowCardinality(String)" -> false
//   - "String" -> false
//
// Use this function in recordMetaFromColumnTypes when processing query results.
// For schema metadata from system.columns, use isNullableType instead.
func isNullableTypeUnwrapped(typeName string) bool {
	// Strip LowCardinality wrapper if present.
	// "LowCardinality(" is 15 characters.
	if len(typeName) > 16 && typeName[:15] == "LowCardinality(" {
		typeName = typeName[15 : len(typeName)-1]
	}

	return isNullableType(typeName)
}

// kindFromClickHouseType maps ClickHouse type names to sq kind.Kind values.
// It handles wrapped types like LowCardinality(T) and Nullable(T) by stripping
// the wrappers to get the underlying base type for kind mapping.
//
// Type mapping:
//
//	ClickHouse Type          -> sq Kind
//	----------------------------------------
//	Int8, Int16, Int32, Int64   -> kind.Int
//	UInt8, UInt16, UInt32, UInt64 -> kind.Int
//	Float32, Float64            -> kind.Float
//	String                      -> kind.Text
//	FixedString(N)              -> kind.Text
//	Bool                        -> kind.Bool
//	Date, Date32                -> kind.Date
//	DateTime, DateTime64        -> kind.Datetime
//	UUID                        -> kind.Text
//	Decimal(P,S)                -> kind.Decimal
//	Array(T)                    -> kind.Text (serialized as text)
//	Unknown types               -> kind.Text (safe fallback)
//
// Wrappers are stripped before mapping:
//   - LowCardinality(Nullable(String)) -> "String" -> kind.Text
//   - Nullable(Int64) -> "Int64" -> kind.Int
func kindFromClickHouseType(chType string) kind.Kind {
	// Strip LowCardinality wrapper if present. Must be done first since
	// LowCardinality can wrap Nullable: LowCardinality(Nullable(String)).
	// "LowCardinality(" is 15 characters.
	if len(chType) > 16 && chType[:15] == "LowCardinality(" {
		chType = chType[15 : len(chType)-1]
	}

	// Strip Nullable wrapper if present. After stripping LowCardinality above,
	// we can use isNullableType which checks for direct Nullable(...) prefix.
	if isNullableType(chType) {
		chType = chType[9 : len(chType)-1]
	}

	switch chType {
	case "UInt8", "UInt16", "UInt32", "UInt64":
		return kind.Int
	case "Int8", "Int16", "Int32", "Int64":
		return kind.Int
	case "Float32", "Float64":
		return kind.Float
	case "String":
		return kind.Text
	case "Bool":
		return kind.Bool
	case "Date", "Date32":
		return kind.Date
	case "DateTime", "DateTime64":
		return kind.Datetime
	case "UUID":
		return kind.Text
	default:
		// Check for FixedString(N) types - ClickHouse returns "FixedString(10)" not "FixedString"
		if len(chType) >= 11 && chType[:11] == "FixedString" {
			return kind.Text
		}
		// Check for Decimal types
		if len(chType) >= 7 && chType[:7] == "Decimal" {
			return kind.Decimal
		}
		// Check for Array types
		if len(chType) >= 5 && chType[:5] == "Array" {
			return kind.Text // Arrays serialized as text for now
		}
		// Default to text for unknown types
		return kind.Text
	}
}

// recordMetaFromColumnTypes creates record metadata from SQL column types
// returned by a query. This metadata is used to properly scan and transform
// query results into sq records.
//
// For each column, it:
//  1. Gets the database type name from sql.ColumnType.DatabaseTypeName()
//  2. Maps the ClickHouse type to an sq kind using kindFromClickHouseType
//  3. Determines nullability using isNullableTypeUnwrapped (handles
//     LowCardinality(Nullable(T)) patterns)
//  4. Sets the appropriate scan type based on kind and nullability
//
// The function uses isNullableTypeUnwrapped rather than isNullableType because
// the ClickHouse driver reports full type strings like
// "LowCardinality(Nullable(String))" where Nullable is not the outermost
// wrapper. See isNullableTypeUnwrapped documentation for details.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		dbTypeName := colType.DatabaseTypeName()
		knd := kindFromClickHouseType(dbTypeName)
		colTypeData := record.NewColumnTypeData(colType, knd)

		// The ClickHouse driver's sql.ColumnType.Nullable() method may not correctly
		// report nullability, so we detect it from DatabaseTypeName(). We use
		// isNullableTypeUnwrapped (not isNullableType) because the driver reports
		// full type strings like "LowCardinality(Nullable(String))" where Nullable
		// is not the outermost wrapper. Without unwrapping, we'd incorrectly treat
		// such columns as non-nullable, causing scan errors when NULL values appear.
		if isNullableTypeUnwrapped(dbTypeName) {
			colTypeData.Nullable = true
			colTypeData.HasNullable = true
		}

		setScanType(colTypeData, knd, colTypeData.Nullable)
		sColTypeData[i] = colTypeData
		ogColNames[i] = colTypeData.Name
	}

	mungedColNames, err := driver.MungeResultColNames(ctx, ogColNames)
	if err != nil {
		return nil, err
	}

	recMeta := make(record.Meta, len(colTypes))
	for i := range sColTypeData {
		recMeta[i] = record.NewFieldMeta(sColTypeData[i], mungedColNames[i])
	}

	return recMeta, nil
}

// getNewRecordFunc returns a NewRecordFunc that transforms scanned row data
// into sq records. The function uses the provided record.Meta to interpret
// the raw scanned values and convert them to the appropriate Go types.
//
// This is used by RecordMeta to provide the transformation function that
// processes each row returned by a query.
func getNewRecordFunc(rowMeta record.Meta) driver.NewRecordFunc {
	return func(row []any) (record.Record, error) {
		rec, _ := driver.NewRecordFromScanRow(rowMeta, row, nil)
		return rec, nil
	}
}

// setScanType sets the appropriate Go reflect.Type for scanning a column's
// values. The scan type determines what Go type will be used to receive values
// from the database driver during row scanning.
//
// For nullable columns, it uses sql.Null* types (e.g., sql.NullString,
// sql.NullInt64) which can represent NULL values. For non-nullable columns,
// it uses the corresponding primitive types (string, int64, etc.).
//
// Scan type mapping:
//
//	Kind        | Nullable         | Non-Nullable
//	------------|------------------|-------------
//	Text        | sql.NullString   | string
//	Int         | sql.NullInt64    | int64
//	Float       | sql.NullFloat64  | float64
//	Bool        | sql.NullBool     | bool
//	Datetime    | sql.NullTime     | time.Time
//	Date        | sql.NullTime     | time.Time
//	Time        | sql.NullTime     | time.Time
//	Decimal     | NullDecimal      | string
//	Bytes       | []byte           | []byte
func setScanType(colTypeData *record.ColumnTypeData, knd kind.Kind, nullable bool) {
	if nullable {
		// Use nullable scan types to properly handle NULL values
		switch knd {
		case kind.Unknown, kind.Null, kind.Text:
			colTypeData.ScanType = sqlz.RTypeNullString
		case kind.Decimal:
			colTypeData.ScanType = sqlz.RTypeNullDecimal
		case kind.Int:
			colTypeData.ScanType = sqlz.RTypeNullInt64
		case kind.Float:
			colTypeData.ScanType = sqlz.RTypeNullFloat64
		case kind.Bool:
			colTypeData.ScanType = sqlz.RTypeNullBool
		case kind.Datetime, kind.Date, kind.Time:
			colTypeData.ScanType = sqlz.RTypeNullTime
		case kind.Bytes:
			colTypeData.ScanType = sqlz.RTypeBytes // []byte handles nil naturally
		}
		return
	}

	// Non-nullable columns use regular scan types
	switch knd {
	case kind.Unknown, kind.Null, kind.Text, kind.Decimal:
		colTypeData.ScanType = sqlz.RTypeString
	case kind.Int:
		colTypeData.ScanType = sqlz.RTypeInt64
	case kind.Float:
		colTypeData.ScanType = sqlz.RTypeFloat64
	case kind.Bool:
		colTypeData.ScanType = sqlz.RTypeBool
	case kind.Datetime, kind.Date, kind.Time:
		colTypeData.ScanType = sqlz.RTypeTime
	case kind.Bytes:
		colTypeData.ScanType = sqlz.RTypeBytes
	}
}
