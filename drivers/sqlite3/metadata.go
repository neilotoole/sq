package sqlite3

import "C"
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// recordMetaFromColumnTypes returns record.Meta for colTypes.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType,
) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		// sqlite is very forgiving at times, e.g. execute
		// a query with a non-existent column name.
		// This can manifest as an empty db type name. This also
		// happens for functions such as COUNT(*).
		dbTypeName := colType.DatabaseTypeName()

		kind := kindFromDBTypeName(ctx, colType.Name(), dbTypeName, colType.ScanType())
		colTypeData := record.NewColumnTypeData(colType, kind)

		// It's necessary to explicitly set the scan type because
		// the backing driver doesn't set it for whatever reason.
		setScanType(ctx, colTypeData) // REVISIT: Legacy? Do we still need this?

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

// setScanType ensures colTypeData.ScanType is set appropriately.
// If the scan type is nil, a scan type will be set based upon
// the col kind. The scan type can be nil in the case where rows.ColumnTypes
// was invoked before rows.Next (this is necessary for an empty table).
//
// If the scan type is NOT a sql.NullTYPE, the corresponding sql.NullTYPE will
// be set.
func setScanType(ctx context.Context, colType *record.ColumnTypeData) {
	scanType, knd := colType.ScanType, colType.Kind

	if scanType != nil {
		// If the scan type is already set, ensure it's sql.NullTYPE.
		switch scanType {
		default:
			// If it's not one of these types, we use "any".
			colType.ScanType = sqlz.RTypeAny
		case sqlz.RTypeInt64:
			colType.ScanType = sqlz.RTypeNullInt64
		case sqlz.RTypeFloat64:
			colType.ScanType = sqlz.RTypeNullFloat64
		case sqlz.RTypeString:
			colType.ScanType = sqlz.RTypeNullString
		case sqlz.RTypeBool:
			colType.ScanType = sqlz.RTypeNullBool
		case sqlz.RTypeTime:
			colType.ScanType = sqlz.RTypeNullTime
		case sqlz.RTypeBytes:
			// no need to change if it's []byte
		}
	}

	switch knd {
	default:
		// Shouldn't happen?
		lg.FromContext(ctx).Warn("Unknown kind for col",
			lga.Col, colType.Name,
			lga.DBType, colType.DatabaseTypeName,
		)
		scanType = sqlz.RTypeAny

	case kind.Text, kind.Decimal:
		scanType = sqlz.RTypeNullString

	case kind.Int:
		scanType = sqlz.RTypeNullInt64

	case kind.Bool:
		scanType = sqlz.RTypeNullBool

	case kind.Float:
		scanType = sqlz.RTypeNullFloat64

	case kind.Bytes:
		scanType = sqlz.RTypeBytes

	case kind.Datetime:
		scanType = sqlz.RTypeNullTime

	case kind.Date:
		scanType = sqlz.RTypeNullTime

	case kind.Time:
		scanType = sqlz.RTypeNullString
	}

	colType.ScanType = scanType
}

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR(64)" -> kind.Text.
// See https://www.sqlite.org/datatype3.html#determination_of_column_affinity
// The scanType arg may be nil (it may not be available to the caller): when
// non-nil it may be used to determine ambiguous cases. For example,
// dbTypeName is empty string for "COUNT(*)"
func kindFromDBTypeName(ctx context.Context, colName, dbTypeName string, scanType reflect.Type) kind.Kind {
	log := lg.FromContext(ctx)
	if dbTypeName == "" {
		// dbTypeName can be empty for functions such as COUNT() etc.
		// But we can infer the type from scanType (if non-nil).
		if scanType == nil {
			// According to the SQLite3 docs:
			//
			//   3. If the declared type for a column contains the
			//      string "BLOB" or **if no type is specified** then the
			//      column has affinity BLOB.
			//
			// However, I'm not certain how significant that claim is. It
			// might be more appropriate to return kind.Unknown here.
			return kind.Bytes
		}

		switch scanType {
		default:
			return kind.Unknown
		case sqlz.RTypeInt64:
			return kind.Int
		case sqlz.RTypeFloat64:
			return kind.Float
		case sqlz.RTypeString:
			return kind.Text
		case sqlz.RTypeBytes:
			return kind.Bytes
		}
	}

	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	// See the examples of type names in the sqlite docs linked above.
	// Given variations such as VARCHAR(255), we first trim the parens
	// parts. Thus VARCHAR(255) becomes VARCHAR.
	i := strings.IndexRune(dbTypeName, '(')
	if i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

	// Try direct matches against common type names
	switch dbTypeName {
	case "INT", "INTEGER", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT", "UNSIGNED BIG INT", "INT2", "INT8":
		knd = kind.Int
	case "REAL", "DOUBLE", "DOUBLE PRECISION", "FLOAT":
		knd = kind.Float
	case "DECIMAL":
		knd = kind.Decimal
	case "TEXT", "CHARACTER", "VARCHAR", "VARYING CHARACTER", "NCHAR", "NATIVE CHARACTER", "NVARCHAR", "CLOB":
		knd = kind.Text
	case "BLOB":
		knd = kind.Bytes
	case "DATETIME", "TIMESTAMP":
		knd = kind.Datetime
	case "DATE":
		knd = kind.Date
	case "TIME":
		knd = kind.Time
	case "BOOLEAN":
		knd = kind.Bool
	case "NUMERIC":
		// NUMERIC is problematic. It could be an int, float, big decimal, etc.
		// kind.Decimal is safest as it can accept any numeric value.
		knd = kind.Decimal
	}

	// If we have a match, return now.
	if knd != kind.Unknown {
		return knd
	}

	// We didn't find an exact match, we'll use the Affinity rules
	// per the SQLite link provided earlier, noting that we default
	// to kind.Text (the docs specify default affinity NUMERIC, which
	// sq handles as kind.Text).
	switch {
	default:
		knd = kind.Unknown
		log.Warn("Unknown SQLite database column type: using alt",
			lga.DBType, dbTypeName,
			lga.Col, colName,
			lga.Kind, knd,
		)
	case strings.Contains(dbTypeName, "INT"):
		knd = kind.Int
	case strings.Contains(dbTypeName, "TEXT"),
		strings.Contains(dbTypeName, "CHAR"),
		strings.Contains(dbTypeName, "CLOB"):
		knd = kind.Text
	case strings.Contains(dbTypeName, "BLOB"):
		knd = kind.Bytes
	case strings.Contains(dbTypeName, "REAL"),
		strings.Contains(dbTypeName, "FLOA"),
		strings.Contains(dbTypeName, "DOUB"):
		knd = kind.Float
	}

	return knd
}

// DBTypeForKind returns the database type for kind.
// For example: Int --> INTEGER
func DBTypeForKind(knd kind.Kind) string {
	switch knd {
	default:
		panic(fmt.Sprintf("unknown kind {%s}", knd))
	case kind.Text, kind.Null, kind.Unknown:
		return "TEXT"
	case kind.Int:
		return "INTEGER"
	case kind.Float:
		return "REAL"
	case kind.Bytes:
		return "BLOB"
	case kind.Decimal:
		return "NUMERIC"
	case kind.Bool:
		return "BOOLEAN"
	case kind.Datetime:
		return "DATETIME"
	case kind.Date:
		return "DATE"
	case kind.Time:
		return "TIME"
	}
}

// getTableMetadata returns metadata for tblName in db.
func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*source.TableMetadata, error) {
	log := lg.FromContext(ctx)
	tblMeta := &source.TableMetadata{Name: tblName}
	// Note that there's no easy way of getting the physical size of
	// a table, so tblMeta.Size remains nil.

	// But we can get the row count and table type ("table" or "view")
	const tpl = `SELECT
(SELECT COUNT(*) FROM %q),
(SELECT type FROM sqlite_master WHERE name = %q LIMIT 1),
(SELECT 1 FROM sqlite_master WHERE name = %q AND substr("sql",0,21) == 'CREATE VIRTUAL TABLE') AS is_virtual,
(SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1)`

	var schema string
	var isVirtualTbl sql.NullBool
	query := fmt.Sprintf(tpl, tblMeta.Name, tblMeta.Name, tblMeta.Name)
	err := db.QueryRowContext(ctx, query).Scan(&tblMeta.RowCount, &tblMeta.DBTableType, &isVirtualTbl, &schema)
	if err != nil {
		return nil, errw(err)
	}

	switch {
	case isVirtualTbl.Valid && isVirtualTbl.Bool:
		tblMeta.TableType = sqlz.TableTypeVirtual
	case tblMeta.DBTableType == sqlz.TableTypeView:
		tblMeta.TableType = sqlz.TableTypeView
	case tblMeta.DBTableType == sqlz.TableTypeTable:
		tblMeta.TableType = sqlz.TableTypeTable
	default:
	}

	tblMeta.FQName = schema + "." + tblName

	// cid	name		type		notnull	dflt_value	pk
	// 0	actor_id	INT			1		<null>		1
	// 1	film_id		INT			1		<null>		2
	// 2	last_update	TIMESTAMP	1		<null>		0
	query = fmt.Sprintf("PRAGMA TABLE_INFO('%s')", tblMeta.Name)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		col := &source.ColMetadata{}
		var notnull int64
		defaultValue := &sql.NullString{}
		pkValue := &sql.NullInt64{}
		err = rows.Scan(&col.Position, &col.Name, &col.BaseType, &notnull, defaultValue, pkValue)
		if err != nil {
			return nil, errw(err)
		}

		if col.BaseType == "" {
			// The TABLE_INFO pragma doesn't return column types for virtual tables.
			//
			// REVISIT: This logic should be pulled out into a separate query for
			// all "untyped" columns, instead of invoking it for every untyped column.
			if col.BaseType, err = getTypeOfColumn(ctx, db, tblMeta.Name, col.Name); err != nil {
				return nil, err
			}
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = defaultValue.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)

		tblMeta.Columns = append(tblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errw(err)
	}

	return tblMeta, nil
}

// getAllTableMetadata gets metadata for each of the
// non-system tables in db's schema. Arg schemaName is used to
// set TableMetadata.FQName; it is not used to select which schema
// to introspect.
func getAllTableMetadata(ctx context.Context, db sqlz.DB, schemaName string) ([]*source.TableMetadata, error) {
	log := lg.FromContext(ctx)
	// This query returns a row for each column of each table,
	// order by table name then col id (ordinal).
	// Results will look like:
	//
	// table_name	type	cid	name		type		"notnull"	dflt_value	pk
	// actor		table	0	actor_id	numeric		1			<null>		1
	// actor		table	1	first_name	VARCHAR(45)	1			<null>		0
	// actor		table	2	last_name	VARCHAR(45)	1			<null>		0
	// actor		table	3	last_update	TIMESTAMP	1			<null>		0
	// address		table	0	address_id	int			1			<null>		1
	// address		table	1	address		VARCHAR(50)	1			<null>		0
	// address		table	2	address2	VARCHAR(50)	0			NULL		0
	// address		table	3	district	VARCHAR(20)	1			<null>		0
	//
	// Note: dflt_value of col "address2" is the string "NULL", rather
	// that NULL value itself.
	const query = `
SELECT m.name as table_name, m.type, p.cid, p.name, p.type, p.'notnull' as 'notnull', p.dflt_value, p.pk,
(substr(m.sql, 0, 21) == 'CREATE VIRTUAL TABLE') AS is_virtual
FROM sqlite_master AS m JOIN pragma_table_info(m.name) AS p
ORDER BY m.name, p.cid
`

	var tblMetas []*source.TableMetadata
	var tblNames []string
	var curTblName string
	var curTblType string
	var curTblIsVirtual bool
	var curTblMeta *source.TableMetadata

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		col := &source.ColMetadata{}
		var notnull int64
		colDefault := &sql.NullString{}
		pkValue := &sql.NullInt64{}

		err = rows.Scan(
			&curTblName,
			&curTblType,
			&col.Position,
			&col.Name,
			&col.BaseType,
			&notnull,
			colDefault,
			pkValue,
			&curTblIsVirtual,
		)
		if err != nil {
			return nil, errw(err)
		}

		if strings.HasPrefix(curTblName, "sqlite_") {
			// Skip system table "sqlite_sequence" etc.
			continue
		}

		if col.BaseType == "" {
			// The TABLE_INFO pragma doesn't return column types for virtual tables.
			//
			// REVISIT: This logic should be pulled out into a separate query for
			// all "untyped" columns, instead of invoking it for every untyped column.
			if col.BaseType, err = getTypeOfColumn(ctx, db, curTblName, col.Name); err != nil {
				return nil, err
			}
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName {
			// On our first time encountering a new table name, we create a new TableMetadata
			curTblMeta = &source.TableMetadata{
				Name:        curTblName,
				FQName:      schemaName + "." + curTblName,
				Size:        nil, // No easy way of getting the storage size of a table
				DBTableType: curTblType,
			}

			switch {
			case curTblIsVirtual:
				curTblMeta.TableType = sqlz.TableTypeVirtual
			case curTblMeta.DBTableType == sqlz.TableTypeView:
				curTblMeta.TableType = sqlz.TableTypeView
			case curTblMeta.DBTableType == sqlz.TableTypeTable:
				curTblMeta.TableType = sqlz.TableTypeTable
			default:
			}

			tblNames = append(tblNames, curTblName)
			tblMetas = append(tblMetas, curTblMeta)
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = colDefault.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)

		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errw(err)
	}

	// Separately, we need to get the row counts for the tables
	var rowCounts []int64
	rowCounts, err = getTblRowCounts(ctx, db, tblNames)
	if err != nil {
		return nil, errw(err)
	}

	for i := range rowCounts {
		tblMetas[i].RowCount = rowCounts[i]
	}

	return tblMetas, nil
}

// getTblRowCounts returns the number of rows in each table.
func getTblRowCounts(ctx context.Context, db sqlz.DB, tblNames []string) ([]int64, error) {
	log := lg.FromContext(ctx)

	// See: https://stackoverflow.com/questions/7524612/how-to-count-rows-from-multiple-tables-in-sqlite
	//
	// Several approaches were benchmarked. Ultimately the union-based
	// query was selected.
	//
	// BenchmarkGetTblRowCounts/benchGetTblRowCountsBaseline-16         	     864	  43631750 ns/op
	// BenchmarkGetTblRowCounts/getTblRowCounts-16                      	    3948	   9126191 ns/op
	//
	// That query looks like:
	//
	//  SELECT COUNT(*) FROM "actor"
	//  UNION ALL
	//  SELECT COUNT(*) FROM "address"
	//  UNION ALL
	//  SELECT COUNT(*) FROM "category"
	//
	// Note that there is a limit (SQLITE_MAX_COMPOUND_SELECT)
	// to the number of "terms" (SELECT clauses) in a query.
	// See https://www.sqlite.org/limits.html#max_compound_select
	//
	// Thus if len(tblNames) > 500, we need to execute multiple queries.
	const maxCompoundSelect = 500

	tblCounts := make([]int64, len(tblNames))

	var sb strings.Builder
	var query string
	var terms int
	var j int

	for i := 0; i < len(tblNames); i++ {
		if terms > 0 {
			sb.WriteString(" UNION ALL ")
		}
		sb.WriteString(fmt.Sprintf("SELECT COUNT(*) FROM %q", tblNames[i]))
		terms++

		if terms != maxCompoundSelect && i != len(tblNames)-1 {
			continue
		}

		query = sb.String()

		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return nil, errw(err)
		}

		for rows.Next() {
			err = rows.Scan(&tblCounts[j])
			if err != nil {
				lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)
				return nil, errw(err)
			}
			j++
		}

		if err = rows.Err(); err != nil {
			lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)
			return nil, errw(err)
		}

		err = rows.Close()
		if err != nil {
			return nil, errw(err)
		}

		terms = 0
		sb.Reset()
	}

	return tblCounts, nil
}

// getTypeOfColumn executes "SELECT typeof(colName)", returning the first result.
// Empty string is returned if there are no rows in that table, as SQLite determines
// type on a per-cell basis, not per-column.
func getTypeOfColumn(ctx context.Context, db sqlz.DB, tblName, colName string) (string, error) {
	colTypeQuery := fmt.Sprintf(`SELECT typeof(%s) FROM %s LIMIT 1`,
		stringz.DoubleQuote(colName), stringz.DoubleQuote(tblName))

	var colType string
	if err := db.QueryRowContext(ctx, colTypeQuery).Scan(&colType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}

		return "", errw(err)
	}

	return colType, nil
}
