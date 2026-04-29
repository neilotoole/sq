package oracle

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// kindFromDBTypeName returns the kind.Kind for the given Oracle database type name.
// When the type name includes precision/scale (e.g. "NUMBER(19,0)" from the data
// dictionary), NUMBER(p,0) with p in [1..19] is mapped to kind.Int; otherwise
// NUMBER is kind.Decimal. Callers that have a bare "NUMBER" type name and access
// to ColumnType.DecimalSize() should refine the result themselves (see RecordMeta).
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	dbTypeName = strings.ToUpper(dbTypeName)

	// Handle NUMBER with embedded precision/scale from the data dictionary
	// (e.g. "NUMBER(19,0)", "NUMBER(10)").
	if strings.HasPrefix(dbTypeName, "NUMBER(") {
		return kindFromOracleNumber(dbTypeName)
	}

	switch dbTypeName {
	case "NUMBER":
		// No precision/scale info available in the type name alone.
		// Callers with access to precision/scale (e.g. via ColumnType.DecimalSize()
		// or data dictionary columns) should refine this to kind.Int when appropriate.
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

// kindFromOracleNumber parses precision and scale from a NUMBER type name that
// already includes them (e.g. "NUMBER(19,0)" or "NUMBER(10)") and returns
// kind.Int for integer-range columns (scale == 0, 1 ≤ precision ≤ 19) or
// kind.Decimal otherwise.
func kindFromOracleNumber(typeName string) kind.Kind {
	// Strip leading "NUMBER(" and trailing ")".
	inner := strings.TrimSuffix(strings.TrimPrefix(typeName, "NUMBER("), ")")
	parts := strings.SplitN(inner, ",", 2)

	precision, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || precision <= 0 || precision > 19 {
		return kind.Decimal
	}

	if len(parts) == 2 {
		scale, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil || scale != 0 {
			return kind.Decimal
		}
	}

	return kind.Int
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
// NOTE: Oracle treats empty string '' as NULL, so we use a single space for text defaults.
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

// renderRowRange renders OFFSET … FETCH … for Oracle 12c+ (no LIMIT/OFFSET).
func renderRowRange(_ *render.Context, rr *ast.RowRangeNode) (string, error) {
	if rr == nil {
		return "", nil
	}

	if rr.Limit < 0 && rr.Offset < 0 {
		return "", nil
	}

	offset := max(rr.Offset, 0)

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("OFFSET %d ROWS", offset))

	if rr.Limit > -1 {
		buf.WriteString(fmt.Sprintf(" FETCH NEXT %d ROWS ONLY", rr.Limit))
	}

	return buf.String(), nil
}

// preRenderOracle ensures ORDER BY exists when a row range is used; Oracle
// requires ORDER BY before OFFSET/FETCH (same pattern as SQL Server).
func preRenderOracle(_ *render.Context, f *render.Fragments) error {
	if f.Range != "" && f.OrderBy == "" {
		f.OrderBy = "ORDER BY (SELECT 0 FROM DUAL)"
	}
	return nil
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
