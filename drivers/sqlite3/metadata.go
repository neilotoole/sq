package sqlite3

import "C"
import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// tableMetadata returns metadata for tblName in db.
func tableMetadata(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) (*source.TableMetadata, error) {
	tblMeta := &source.TableMetadata{Name: tblName}
	tblMeta.Size = -1 // No easy way of getting size of table, so set to -1

	// But we can get the row count
	query := fmt.Sprintf("SELECT COUNT(*) FROM '%s'", tblMeta.Name)
	row := db.QueryRowContext(ctx, query)
	err := row.Scan(&tblMeta.RowCount)
	if err != nil {
		return nil, errz.Err(err)
	}

	// cid	name		type		notnull	dflt_value	pk
	// 0	actor_id	INT			1		<null>		1
	// 1	film_id		INT			1		<null>		2
	// 2	last_update	TIMESTAMP	1		<null>		0
	query = fmt.Sprintf("PRAGMA TABLE_INFO('%s')", tblMeta.Name)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	for rows.Next() {
		col := &source.ColMetadata{}
		var notnull int64
		defaultValue := &sql.NullString{}
		pkValue := &sql.NullInt64{}
		err = rows.Scan(&col.Position, &col.Name, &col.BaseType, &notnull, defaultValue, pkValue)
		if err != nil {
			return nil, errz.Err(err)
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = defaultValue.String
		col.Kind = kindFromDBTypeName(log, col.Name, col.BaseType, nil)

		tblMeta.Columns = append(tblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	return tblMeta, nil
}

// recordMetaFromColumnTypes returns recordMetaFromColumnTypes for rows.
func recordMetaFromColumnTypes(log lg.Log, colTypes []*sql.ColumnType) (sqlz.RecordMeta, error) {
	recMeta := make([]*sqlz.FieldMeta, len(colTypes))
	for i, colType := range colTypes {
		// sqlite is very forgiving at times, e.g. execute
		// a query with a non-existent column name.
		// This can manifest as an empty db type name. This also
		// happens for functions such as COUNT(*).
		dbTypeName := colType.DatabaseTypeName()

		kind := kindFromDBTypeName(log, colType.Name(), dbTypeName, colType.ScanType())
		colTypeData := sqlz.NewColumnTypeData(colType, kind)

		// It's necessary to explicitly set the scan type because
		// the backing driver doesn't set it for whatever reason.
		setScanType(log, colTypeData)
		recMeta[i] = sqlz.NewFieldMeta(colTypeData)
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
func setScanType(log lg.Log, colType *sqlz.ColumnTypeData) {
	scanType, kind := colType.ScanType, colType.Kind

	if scanType != nil {
		// If the scan type is already set, ensure it's sql.NullTYPE.
		switch scanType {
		default:
			// It's already a sql.NullTYPE.
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

	switch kind {
	default:
		// Shouldn't happen?
		log.Warnf("Unknown kind for col '%s' with database type '%s'", colType.Name, colType.DatabaseTypeName)
		scanType = sqlz.RTypeBytes

	case sqlz.KindText, sqlz.KindDecimal:
		scanType = sqlz.RTypeNullString

	case sqlz.KindInt:
		scanType = sqlz.RTypeNullInt64

	case sqlz.KindBool:
		scanType = sqlz.RTypeNullBool

	case sqlz.KindFloat:
		scanType = sqlz.RTypeNullFloat64

	case sqlz.KindBytes:
		scanType = sqlz.RTypeBytes

	case sqlz.KindDatetime:
		scanType = sqlz.RTypeNullTime

	case sqlz.KindDate:
		scanType = sqlz.RTypeNullTime

	case sqlz.KindTime:
		scanType = sqlz.RTypeNullString
	}

	colType.ScanType = scanType
}

// kindFromDBTypeName determines the sqlz.Kind from the database
// type name. For example, "VARCHAR(64)" -> sqlz.KindText.
// See https://www.sqlite.org/datatype3.html#determination_of_column_affinity
// The scanType arg may be nil (it may not be available to the caller): when
// non-nil it may be used to determine ambiguous cases. For example,
// dbTypeName is empty string for "COUNT(*)"
func kindFromDBTypeName(log lg.Log, colName, dbTypeName string, scanType reflect.Type) sqlz.Kind {
	if dbTypeName == "" {
		// dbTypeName can be empty for functions such as COUNT() etc.
		// But we can infer the type from scanType (if non-nil).
		if scanType == nil {
			// According to the SQLite3 docs:
			//   3. If the declared type for a column contains the
			//      string "BLOB" or **if no type is specified** then the
			//      column has affinity BLOB.
			return sqlz.KindBytes
		}

		switch scanType {
		default:
			// Default to KindBytes as mentioned above.
			return sqlz.KindBytes
		case sqlz.RTypeInt64:
			return sqlz.KindInt
		case sqlz.RTypeFloat64:
			return sqlz.KindFloat
		case sqlz.RTypeString:
			return sqlz.KindText
		}
	}

	var kind sqlz.Kind
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
		kind = sqlz.KindInt
	case "REAL", "DOUBLE", "DOUBLE PRECISION", "FLOAT":
		kind = sqlz.KindFloat
	case "DECIMAL":
		kind = sqlz.KindDecimal
	case "TEXT", "CHARACTER", "VARCHAR", "VARYING CHARACTER", "NCHAR", "NATIVE CHARACTER", "NVARCHAR", "CLOB":
		kind = sqlz.KindText
	case "BLOB":
		kind = sqlz.KindBytes
	case "DATETIME", "TIMESTAMP":
		kind = sqlz.KindDatetime
	case "DATE":
		kind = sqlz.KindDate
	case "TIME":
		kind = sqlz.KindTime
	case "BOOLEAN":
		kind = sqlz.KindBool
	case "NUMERIC":
		// NUMERIC is problematic. It could be an int, float, big decimal, etc.
		// KindDecimal is safest as it can accept any numeric value.
		kind = sqlz.KindDecimal
	}

	// If we have a match, return now.
	if kind != sqlz.KindUnknown {
		return kind
	}

	// We didn't find an exact match, we'll use the Affinity rules
	// per the SQLite link provided earlier, noting that we default
	// to KindText (the docs specify default affinity NUMERIC, which
	// sq handles as KindText).
	switch {
	default:
		log.Warnf("Unknown SQLite database type name %q for %q: using %q", dbTypeName, colName, sqlz.KindUnknown)
		kind = sqlz.KindUnknown
	case strings.Contains(dbTypeName, "INT"):
		kind = sqlz.KindInt
	case strings.Contains(dbTypeName, "TEXT"),
		strings.Contains(dbTypeName, "CHAR"),
		strings.Contains(dbTypeName, "CLOB"):
		kind = sqlz.KindText
	case strings.Contains(dbTypeName, "BLOB"):
		kind = sqlz.KindBytes
	case strings.Contains(dbTypeName, "REAL"),
		strings.Contains(dbTypeName, "FLOA"),
		strings.Contains(dbTypeName, "DOUB"):
		kind = sqlz.KindFloat
	}

	return kind
}

// DBTypeForKind returns the database type for kind.
// For example: KindInt --> INTEGER
func DBTypeForKind(kind sqlz.Kind) string {
	switch kind {
	default:
		panic(fmt.Sprintf("unknown kind %q", kind))
	case sqlz.KindText, sqlz.KindNull, sqlz.KindUnknown:
		return "TEXT"
	case sqlz.KindInt:
		return "INTEGER"
	case sqlz.KindFloat:
		return "REAL"
	case sqlz.KindBytes:
		return "BLOB"
	case sqlz.KindDecimal:
		return "NUMERIC"
	case sqlz.KindBool:
		return "BOOLEAN"
	case sqlz.KindDatetime:
		return "DATETIME"
	case sqlz.KindDate:
		return "DATE"
	case sqlz.KindTime:
		return "TIME"
	}
}

// getAllTblMeta gets metadata for each of the
// non-system tables in db.
func getAllTblMeta(ctx context.Context, log lg.Log, db sqlz.DB) ([]*source.TableMetadata, error) {
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
SELECT m.name as table_name, m.type, p.cid, p.name, p.type, p.'notnull' as 'notnull', p.dflt_value, p.pk
FROM sqlite_master AS m JOIN pragma_table_info(m.name) AS p
ORDER BY m.name, p.cid
`

	var tblMetas []*source.TableMetadata
	var tblNames []string
	var curTblName string
	var curTblType string // either "table" or "view"
	var curTblMeta *source.TableMetadata

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		col := &source.ColMetadata{}
		var notnull int64
		defaultValue := &sql.NullString{}
		pkValue := &sql.NullInt64{}

		err = rows.Scan(&curTblName, &curTblType, &col.Position, &col.Name, &col.BaseType, &notnull, defaultValue, pkValue)
		if err != nil {
			return nil, errz.Err(err)
		}

		if strings.HasPrefix(curTblName, "sqlite_") {
			// Skip system table "sqlite_sequence" etc.
			continue
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName {
			// On our first time encountering a new table name, we create a new TableMetadata
			curTblMeta = &source.TableMetadata{
				Name:      curTblName,
				Size:      -1, // No easy way of getting the storage size of a table
				TableType: curTblType,
			}

			tblNames = append(tblNames, curTblName)
			tblMetas = append(tblMetas, curTblMeta)
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = defaultValue.String
		col.Kind = kindFromDBTypeName(log, col.Name, col.BaseType, nil)

		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	// Separately, we need to get the row counts for the tables
	var rowCounts []int64
	rowCounts, err = getTblRowCounts(ctx, log, db, tblNames)
	if err != nil {
		return nil, errz.Err(err)
	}

	for i := range rowCounts {
		tblMetas[i].RowCount = rowCounts[i]
	}

	return tblMetas, nil
}

// getTblRowCounts returns the number of rows in each table.
func getTblRowCounts(ctx context.Context, log lg.Log, db sqlz.DB, tblNames []string) ([]int64, error) {
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
			return nil, errz.Err(err)
		}

		for rows.Next() {
			err = rows.Scan(&tblCounts[j])
			if err != nil {
				log.WarnIfCloseError(rows)
				return nil, errz.Err(err)
			}
			j++
		}

		if err = rows.Err(); err != nil {
			log.WarnIfCloseError(rows)
			return nil, errz.Err(err)
		}

		err = rows.Close()
		if err != nil {
			return nil, errz.Err(err)
		}

		terms = 0
		sb.Reset()
	}

	return tblCounts, nil
}
