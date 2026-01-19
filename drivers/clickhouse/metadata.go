package clickhouse

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// getSourceMetadata returns metadata for the ClickHouse source.
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

// getTablesMetadata returns metadata for all tables in the database.
func getTablesMetadata(ctx context.Context, db *sql.DB, dbName string) ([]*metadata.Table, error) {
	const query = `
		SELECT
			name,
			engine,
			total_rows,
			total_bytes
		FROM system.tables
		WHERE database = ?
		  AND engine NOT IN ('View', 'MaterializedView')
		ORDER BY name
	`

	rows, err := db.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, errw(err)
	}
	defer rows.Close()

	var tables []*metadata.Table
	for rows.Next() {
		var tblName, engine string
		var totalRows, totalBytes sql.NullInt64

		if err = rows.Scan(&tblName, &engine, &totalRows, &totalBytes); err != nil {
			return nil, errw(err)
		}

		tblMeta := &metadata.Table{
			Name:      tblName,
			TableType: "table",
			RowCount:  totalRows.Int64,
		}

		if totalBytes.Valid {
			bytes := totalBytes.Int64
			tblMeta.Size = &bytes
		}

		// Get column metadata for this table
		cols, colErr := getColumnsMetadata(ctx, db, dbName, tblName)
		if colErr != nil {
			// Log error but continue with other tables
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

// getTableMetadata returns metadata for a specific table.
func getTableMetadata(ctx context.Context, db *sql.DB, dbName, tblName string) (*metadata.Table, error) {
	// If dbName is empty, use currentDatabase()
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
		Name:      tblName,
		TableType: "table",
		RowCount:  totalRows.Int64,
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

// getColumnsMetadata returns metadata for all columns in a table.
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
	defer rows.Close()

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
			Nullable:   isNullableType(colType),
			Comment:    comment.String,
		}

		// ClickHouse doesn't have traditional primary keys in the SQL sense
		// The ORDER BY clause defines sort order, not uniqueness
		col.PrimaryKey = false

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// isNullableType checks if a ClickHouse type is nullable.
// This only checks the outermost wrapper; use isNullableTypeUnwrapped
// to handle LowCardinality(Nullable(T)) cases.
func isNullableType(typeName string) bool {
	// Nullable types are wrapped in Nullable(...)
	return len(typeName) > 9 && typeName[:9] == "Nullable("
}

// isNullableTypeUnwrapped checks if a ClickHouse type is nullable,
// after stripping any LowCardinality wrapper. This correctly handles
// both Nullable(T) and LowCardinality(Nullable(T)) patterns.
func isNullableTypeUnwrapped(typeName string) bool {
	// Strip LowCardinality wrapper if present
	// "LowCardinality(" is 15 characters
	if len(typeName) > 16 && typeName[:15] == "LowCardinality(" {
		typeName = typeName[15 : len(typeName)-1]
	}

	return isNullableType(typeName)
}

// kindFromClickHouseType maps ClickHouse type names to sq kinds.
func kindFromClickHouseType(chType string) kind.Kind {
	// Strip LowCardinality wrapper if present (check first, as it may wrap Nullable)
	// "LowCardinality(" is 15 characters
	if len(chType) > 16 && chType[:15] == "LowCardinality(" {
		chType = chType[15 : len(chType)-1]
	}

	// Strip Nullable wrapper if present
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
	case "String", "FixedString":
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

// recordMetaFromColumnTypes creates record metadata from SQL column types.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		dbTypeName := colType.DatabaseTypeName()
		knd := kindFromClickHouseType(dbTypeName)
		colTypeData := record.NewColumnTypeData(colType, knd)

		// The ClickHouse driver may not report Nullable correctly via sql.ColumnType.Nullable(),
		// so we detect it from the database type name. This handles both Nullable(T) and
		// LowCardinality(Nullable(T)) patterns.
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

// getNewRecordFunc returns a NewRecordFunc for ClickHouse.
func getNewRecordFunc(rowMeta record.Meta) driver.NewRecordFunc {
	return func(row []any) (record.Record, error) {
		rec, _ := driver.NewRecordFromScanRow(rowMeta, row, nil)
		return rec, nil
	}
}

// setScanType sets the appropriate scan type for a column.
// For nullable columns, it uses the nullable scan types (e.g., sql.NullString)
// to properly handle NULL values.
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
