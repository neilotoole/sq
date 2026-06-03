package rqlite

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
)

// recordMetaFromColumnTypes builds record.Meta for colTypes returned by
// gorqlite. The shape matches the sqlite3 driver's helper: the SQL is
// SQLite's, so the column-type names and affinity rules apply verbatim.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		dbTypeName := colType.DatabaseTypeName()
		knd := kindFromDBTypeName(ctx, colType.Name(), dbTypeName, colType.ScanType())
		colTypeData := record.NewColumnTypeData(colType, knd)
		setScanType(ctx, colTypeData)
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

// setScanType normalizes colType.ScanType to the appropriate
// sql.NullTYPE (or rtypeNullTime, for time-kind columns). If the
// driver-supplied scan type is nil — which gorqlite produces for some
// column shapes — the destination is chosen from colType.Kind alone.
func setScanType(ctx context.Context, colType *record.ColumnTypeData) {
	scanType, knd := colType.ScanType, colType.Kind

	if scanType != nil {
		switch scanType {
		default:
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
			// no change.
		}
	}

	switch knd { //nolint:exhaustive
	default:
		lg.FromContext(ctx).Warn("Unknown kind for col",
			lga.Col, colType.Name,
			lga.DBType, colType.DatabaseTypeName,
		)
		scanType = sqlz.RTypeAny
	case kind.Text:
		scanType = sqlz.RTypeNullString
	case kind.Decimal:
		scanType = sqlz.RTypeNullDecimal
	case kind.Int:
		scanType = sqlz.RTypeNullInt64
	case kind.Bool:
		scanType = sqlz.RTypeNullBool
	case kind.Float:
		scanType = sqlz.RTypeNullFloat64
	case kind.Bytes:
		scanType = sqlz.RTypeBytes
	case kind.Datetime:
		scanType = rtypeNullTime
	case kind.Date:
		scanType = rtypeNullTime
	case kind.Time:
		scanType = sqlz.RTypeNullString
	}

	colType.ScanType = scanType
}

// kindFromDBTypeName resolves a SQLite-affinity column type to its
// kind.Kind. The rules are SQLite's, so the implementation is a copy
// of the sqlite3 driver's. See
// https://www.sqlite.org/datatype3.html#determination_of_column_affinity.
//
// dbTypeName may be empty (e.g. for COUNT(*)); scanType, when non-nil,
// is used to break the tie.
func kindFromDBTypeName(ctx context.Context, colName, dbTypeName string, scanType reflect.Type) kind.Kind {
	log := lg.FromContext(ctx)
	if dbTypeName == "" {
		if scanType == nil {
			// Per the SQLite docs, a column with no declared type has
			// affinity BLOB; that's the safest fallback when we have
			// no scan-type hint either.
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

	// Strip any parameterized suffix (e.g. "VARCHAR(255)" -> "VARCHAR").
	if i := strings.IndexRune(dbTypeName, '('); i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

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
		// NUMERIC could be int, float, or big decimal; Decimal is the
		// safest sink because it accepts any numeric value.
		knd = kind.Decimal
	}

	if knd != kind.Unknown {
		return knd
	}

	// Fall back to SQLite affinity rules. SQLite's default affinity is
	// NUMERIC; sq surfaces that as kind.Text.
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

// DBTypeForKind returns the SQLite database type for kind.
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

// newRecordFromScanRow converts a Scan row into a record.Record. The
// shape mirrors the sqlite3 driver's helper, minus a few cases that
// only fire for SQL drivers that report richer types than gorqlite
// does (decimal.Decimal, sqlz.NullBool from sqlserver). The remaining
// cases are exercised when scan destinations set by setScanType
// (sql.Null* wrappers and *nullTime) are unwrapped here.
//
//nolint:gocognit
func newRecordFromScanRow(meta record.Meta, row []any) (rec record.Record) {
	rec = make([]any, len(row))

	for i := 0; i < len(row); i++ {
		if row[i] == nil {
			rec[i] = nil
			continue
		}

		col := row[i]
		if ptr, ok := col.(*any); ok {
			col = *ptr
		}

		switch col := col.(type) {
		default:
			rec[i] = col
			continue
		case nil:
			rec[i] = nil
		case *int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = *col
		case int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = col
		case *float64:
			record.SetKindIfUnknown(meta, i, kind.Float)
			rec[i] = *col
		case float64:
			record.SetKindIfUnknown(meta, i, kind.Float)
			rec[i] = col
		case *bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = *col
		case bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = col
		case *string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = *col
		case string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = col
		case *[]byte:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}
			if meta[i].Kind() != kind.Bytes {
				rec[i] = string(*col)
				record.SetKindIfUnknown(meta, i, kind.Text)
				continue
			}
			if len(*col) == 0 {
				rec[i] = []byte{}
			} else {
				dest := make([]byte, len(*col))
				copy(dest, *col)
				rec[i] = dest
			}
			record.SetKindIfUnknown(meta, i, kind.Bytes)
		case *sql.NullInt64:
			if col.Valid {
				rec[i] = col.Int64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Int)
		case *sql.NullString:
			if col.Valid {
				rec[i] = col.String
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Text)
		case *sql.RawBytes:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}
			knd := meta[i].Kind()
			if len(*col) == 0 {
				if knd == kind.Bytes {
					rec[i] = []byte{}
				} else {
					var s string
					rec[i] = s
					record.SetKindIfUnknown(meta, i, kind.Text)
				}
				continue
			}
			dest := make([]byte, len(*col))
			copy(dest, *col)
			if knd == kind.Bytes {
				rec[i] = dest
			} else {
				rec[i] = string(dest)
				record.SetKindIfUnknown(meta, i, kind.Text)
			}
		case *sql.NullFloat64:
			if col.Valid {
				rec[i] = col.Float64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Float)
		case *sql.NullBool:
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Bool)
		case *sql.NullTime:
			if col.Valid {
				rec[i] = col.Time
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case *nullTime:
			// No SetKindIfUnknown call here: *nullTime is only allocated
			// for columns already classified as kind.Datetime/Date, so
			// the kind is never unknown at this point.
			switch {
			case !col.Valid:
				rec[i] = nil
			case col.IsTime:
				rec[i] = col.Time
			default:
				rec[i] = col.String
			}
		case *time.Time:
			rec[i] = *col
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case time.Time:
			rec[i] = col
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		}
	}

	return rec
}
