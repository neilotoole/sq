package oracle

import (
	"context"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// kindFromDBTypeName returns the kind.Kind for the given Oracle database type name.
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	case "NUMBER":
		// NUMBER can be Int or Decimal depending on precision/scale
		// Default to Decimal, refine in setScanType based on precision/scale
		return kind.Decimal
	case "VARCHAR2", "NVARCHAR2", "CHAR", "NCHAR":
		return kind.Text
	case "CLOB", "NCLOB":
		return kind.Text
	case "BLOB":
		return kind.Bytes
	case "RAW", "LONG RAW":
		return kind.Bytes
	case "DATE":
		// Oracle DATE includes time (equivalent to DATETIME)
		return kind.Datetime
	case "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE":
		return kind.Datetime
	case "BINARY_FLOAT", "BINARY_DOUBLE", "FLOAT":
		return kind.Float
	case "INTERVAL DAY TO SECOND", "INTERVAL YEAR TO MONTH":
		return kind.Text
	default:
		if log != nil {
			log.Warn("Unknown Oracle column type",
				"db_type", dbTypeName,
				"column", colName,
				"defaulting_to", kind.Unknown)
		}
		return kind.Unknown
	}
}

// dbTypeNameFromKind returns the Oracle database type name for the given kind.Kind.
func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd {
	case kind.Null, kind.Text:
		return "VARCHAR2(4000)"
	case kind.Int:
		return "NUMBER(19,0)"
	case kind.Float:
		return "BINARY_DOUBLE"
	case kind.Decimal:
		return "NUMBER"
	case kind.Bool:
		// Oracle has no native BOOLEAN type, use NUMBER(1,0)
		return "NUMBER(1,0)"
	case kind.Datetime:
		return "TIMESTAMP"
	case kind.Time:
		// Oracle has no standalone TIME type, use TIMESTAMP
		return "TIMESTAMP"
	case kind.Date:
		return "DATE"
	case kind.Bytes:
		return "BLOB"
	case kind.Unknown:
		return "VARCHAR2(4000)"
	}
	return "VARCHAR2(4000)"
}

// createTblKindDefaults is a map of kind.Kind to default value for CREATE TABLE.
// NOTE: Oracle treats empty string ” as NULL, so we use a single space for text defaults.
// Oracle also doesn't support function calls (like EMPTY_BLOB()) as DEFAULT values,
// so BLOB columns with NOT NULL must be handled without a default.
var createTblKindDefaults = map[kind.Kind]string{
	kind.Null:     "",            // NULL kind has no default
	kind.Text:     "DEFAULT ' '", // Oracle treats '' as NULL, use space instead
	kind.Int:      "DEFAULT 0",
	kind.Float:    "DEFAULT 0",
	kind.Decimal:  "DEFAULT 0",
	kind.Bool:     "DEFAULT 0",
	kind.Datetime: "DEFAULT TIMESTAMP '1970-01-01 00:00:00'",
	kind.Date:     "DEFAULT DATE '1970-01-01'",
	kind.Time:     "DEFAULT TIMESTAMP '1970-01-01 00:00:00'",
	kind.Bytes:    "",            // Oracle doesn't support EMPTY_BLOB() as DEFAULT; omit default
	kind.Unknown:  "DEFAULT ' '", // Oracle treats '' as NULL, use space instead
}

// buildCreateTableStmt builds a CREATE TABLE statement for Oracle.
func buildCreateTableStmt(tblDef *schema.Table) string {
	sb := strings.Builder{}
	sb.WriteString(`CREATE TABLE "`)
	sb.WriteString(strings.ToUpper(tblDef.Name))
	sb.WriteString(`" (`)

	for i, colDef := range tblDef.Cols {
		sb.WriteString("\n  \"")
		sb.WriteString(strings.ToUpper(colDef.Name))
		sb.WriteString("\" ")
		sb.WriteString(dbTypeNameFromKind(colDef.Kind))

		if colDef.NotNull {
			// Add default value if one exists for this kind
			// (some types like BLOB don't have valid defaults in Oracle)
			if defaultVal := createTblKindDefaults[colDef.Kind]; defaultVal != "" {
				sb.WriteRune(' ')
				sb.WriteString(defaultVal)
			}
			sb.WriteString(" NOT NULL")
		}

		if i < len(tblDef.Cols)-1 {
			sb.WriteRune(',')
		}
	}

	sb.WriteString("\n)")
	return sb.String()
}

// CreateTable creates a table in Oracle.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	stmt := buildCreateTableStmt(tblDef)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}
