package sqlserver

import (
	"database/sql"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/sqlz"
)

// kindFromDBTypeName determines the sqlz.Kind from the database
// type name. For example, "VARCHAR" -> sqlz.KindText.
func kindFromDBTypeName(log lg.Log, colName, dbTypeName string) sqlz.Kind {
	var kind sqlz.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	default:
		log.Warnf("Unknown SQLServer database type '%s' for column '%s': using %s", dbTypeName, colName, sqlz.KindUnknown)
		kind = sqlz.KindUnknown
	case "INT", "BIGINT", "SMALLINT", "TINYINT":
		kind = sqlz.KindInt
	case "CHAR", "NCHAR", "VARCHAR", "JSON", "NVARCHAR", "NTEXT", "TEXT":
		kind = sqlz.KindText
	case "BIT":
		kind = sqlz.KindBool
	case "BINARY", "VARBINARY", "IMAGE":
		kind = sqlz.KindBytes
	case "DECIMAL", "NUMERIC":
		kind = sqlz.KindDecimal
	case "MONEY", "SMALLMONEY":
		kind = sqlz.KindDecimal
	case "DATETIME", "DATETIME2", "SMALLDATETIME", "DATETIMEOFFSET":
		kind = sqlz.KindDatetime
	case "DATE":
		kind = sqlz.KindDate
	case "TIME":
		kind = sqlz.KindTime
	case "FLOAT", "REAL":
		kind = sqlz.KindFloat
	case "XML":
		kind = sqlz.KindText
	case "UNIQUEIDENTIFIER":
		kind = sqlz.KindText
	case "ROWVERSION", "TIMESTAMP":
		kind = sqlz.KindInt
	}

	return kind
}

// setScanType does some manipulation of ct's scan type.
// Most importantly, if ct is nullable column, setwe  colTypeData.ScanType to a
// nullable type. This is because the driver doesn't
// report nullable scan types.
func setScanType(ct *sqlz.ColumnTypeData, kind sqlz.Kind) {
	if kind == sqlz.KindDecimal {
		// The driver wants us to use []byte instead of string for DECIMAL,
		// but we want to use string.
		if ct.Nullable {
			ct.ScanType = sqlz.RTypeNullString
		} else {
			ct.ScanType = sqlz.RTypeString
		}
		return
	}

	if !ct.Nullable {
		// If the col type is not nullable, there's nothing
		// to do here.
		return
	}

	switch ct.ScanType {
	default:
		ct.ScanType = sqlz.RTypeNullString

	case sqlz.RTypeInt64:
		ct.ScanType = sqlz.RTypeNullInt64

	case sqlz.RTypeBool:
		ct.ScanType = sqlz.RTypeNullBool

	case sqlz.RTypeFloat64:
		ct.ScanType = sqlz.RTypeNullFloat64

	case sqlz.RTypeString:
		ct.ScanType = sqlz.RTypeNullString

	case sqlz.RTypeTime:
		ct.ScanType = sqlz.RTypeNullTime

	case sqlz.RTypeBytes:
		ct.ScanType = sqlz.RTypeBytes // no change
	}
}

// SchemaConstraint models SQL Server constraints, e.g. "PRIMARY KEY", "FOREIGN KEY", etc.
type SchemaConstraint struct {
	TableCatalog   string `db:"TABLE_CATALOG"`
	TableSchema    string `db:"TABLE_SCHEMA"`
	TableName      string `db:"TABLE_NAME"`
	ConstraintType string `db:"CONSTRAINT_TYPE"`
	ColumnName     string `db:"COLUMN_NAME"`
	ConstraintName string `db:"CONSTRAINT_NAME"`
}

// SchemaColumn models SQL Server's INFORMATION_SCHEMA.COLUMNS table.
type SchemaColumn struct {
	TableCatalog          string         `db:"TABLE_CATALOG"`
	TableSchema           string         `db:"TABLE_SCHEMA"`
	TableName             string         `db:"TABLE_NAME"`
	ColumnName            string         `db:"COLUMN_NAME"`
	OrdinalPosition       int64          `db:"ORDINAL_POSITION"`
	ColumnDefault         sql.NullString `db:"COLUMN_DEFAULT"`
	Nullable              sqlz.NullBool  `db:"IS_NULLABLE"`
	DataType              string         `db:"DATA_TYPE"`
	CharMaxLength         sql.NullInt64  `db:"CHARACTER_MAXIMUM_LENGTH"`
	CharOctetLength       sql.NullString `db:"CHARACTER_OCTET_LENGTH"`
	NumericPrecision      sql.NullInt64  `db:"NUMERIC_PRECISION"`
	NumericPrecisionRadix sql.NullInt64  `db:"NUMERIC_PRECISION_RADIX"`
	NumericScale          sql.NullInt64  `db:"NUMERIC_SCALE"`
	DateTimePrecision     sql.NullInt64  `db:"DATETIME_PRECISION"`
	CharSetCatalog        sql.NullString `db:"CHARACTER_SET_CATALOG"`
	CharSetSchema         sql.NullString `db:"CHARACTER_SET_SCHEMA"`
	CharSetName           sql.NullString `db:"CHARACTER_SET_NAME"`
	CollationCatalog      sql.NullString `db:"COLLATION_CATALOG"`
	CollationSchema       sql.NullString `db:"COLLATION_SCHEMA"`
	CollationName         sql.NullString `db:"COLLATION_NAME"`
	DomainCatalog         sql.NullString `db:"DOMAIN_CATALOG"`
	DomainSchema          sql.NullString `db:"DOMAIN_SCHEMA"`
	DomainName            sql.NullString `db:"DOMAIN_NAME"`
}
