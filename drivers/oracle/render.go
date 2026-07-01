package oracle

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// kindFromDBTypeName returns the kind.Kind for the given Oracle database type name.
// Parameter groups are stripped from the type name before matching, so callers
// may pass either the bare form ("VARCHAR2", "TIMESTAMP") or the parameterized
// form from the data dictionary ("VARCHAR2(91)", "TIMESTAMP(6) WITH TIME ZONE",
// "INTERVAL DAY(2) TO SECOND(6)"). NUMBER is special-cased: when the type name
// includes precision/scale (e.g. "NUMBER(19,0)"), NUMBER(p,0) with p in [1..19]
// is mapped to kind.Int; otherwise NUMBER is kind.Decimal. Callers that have a
// bare "NUMBER" type name and access to ColumnType.DecimalSize() should refine
// the result themselves (see RecordMeta).
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	dbTypeName = strings.ToUpper(dbTypeName)

	// NUMBER's kind depends on its precision/scale, so it's parsed
	// specially before the generic param-strip below.
	if strings.HasPrefix(dbTypeName, "NUMBER(") {
		return kindFromOracleNumber(dbTypeName)
	}

	// Strip parameter parens so e.g. "VARCHAR2(91)" or
	// "TIMESTAMP(6) WITH TIME ZONE" match their bare form below.
	dbTypeName = stripTypeParams(dbTypeName)

	switch dbTypeName {
	case "NUMBER":
		// No precision/scale info available in the type name alone.
		// Callers with access to precision/scale (e.g. via ColumnType.DecimalSize()
		// or data dictionary columns) should refine this to kind.Int when appropriate.
		return kind.Decimal
	case "VARCHAR2", "NVARCHAR2", "CHAR", "NCHAR", "VARCHAR", "LONG", "LONGVARCHAR":
		return kind.Text
	case "CLOB", "NCLOB", "OCICLOBLOCATOR", "OCISTRING":
		return kind.Text
	case "BLOB", "OCIBLOBLOCATOR", "OCIFILELOCATOR":
		return kind.Bytes
	case "RAW", "LONG RAW", "VARRAW", "LONGRAW", "LONGVARRAW":
		return kind.Bytes
	case "DATE", "OCIDATE":
		// Oracle DATE includes time (equivalent to DATETIME)
		return kind.Datetime
	// Data-dictionary forms come from USER_TAB_COLUMNS; the *DTY variants
	// come from the go-ora wire driver's ColumnType.DatabaseTypeName().
	case "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE",
		"TIMESTAMPDTY", "TIMESTAMPTZ", "TIMESTAMPTZ_DTY",
		"TIMESTAMPLTZ_DTY", "TIMESTAMPELTZ":
		return kind.Datetime
	case "TIMETZ":
		return kind.Time
	case "BINARY_FLOAT", "BINARY_DOUBLE", "FLOAT",
		"BFLOAT", "BDOUBLE", "IBFLOAT", "IBDOUBLE":
		return kind.Float
	case "INTERVAL DAY TO SECOND", "INTERVAL YEAR TO MONTH",
		"INTERVALYM", "INTERVALDS", "INTERVALYM_DTY", "INTERVALDS_DTY":
		return kind.Text
	case "ROWID", "UROWID":
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

// stripTypeParams removes parenthesized parameter groups from a type name and
// collapses the resulting whitespace. It handles parens that appear in the
// middle of multi-word Oracle type names (e.g. "TIMESTAMP(6) WITH TIME ZONE",
// "INTERVAL DAY(2) TO SECOND(6)").
func stripTypeParams(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	depth := 0
	for _, r := range s {
		switch {
		case r == '(':
			depth++
		case r == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
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

// oracleScaleFloating is the Oracle wire-protocol sentinel for "no scale
// specified" — the uint8 representation of signed int8 -1. It appears on
// floating NUMBER results: COUNT(*), SUM, integer literals, and arithmetic
// where Oracle does not pin down a scale.
const oracleScaleFloating = 255

// refineBareNumberKind refines a kind.Decimal classification for a bare-NUMBER
// column using the precision/scale info from sql.ColumnType.DecimalSize(). It
// returns kind.Int only when the precision/scale pin the column to the integer
// range, and kind.Decimal otherwise (the safe default).
//
// The floating-scale form NUMBER(38, oracleScaleFloating) is ambiguous: COUNT(*),
// SUM, AVG, integer literals, division, and other arithmetic all report it, and
// Oracle won't say whether the value is integral. Classifying it as kind.Int
// crashes the scan when the value is fractional (e.g. a division like
// actor_id/8 yields "7.25", which won't parse into int64). So the floating-scale
// form maps to kind.Decimal, which is exact and scans any value without error,
// for both SLQ and native sq sql. See issue #844.
//
// count(), count_unique(), and rownum() are integer-valued but share this
// ambiguous form; they are pinned back to kind.Int via Renderer.FunctionResultKinds
// (see Renderer), which RecordMeta applies. A bare NUMBER(p, 0) with p in [1..19]
// is unambiguously int-range and stays kind.Int.
func refineBareNumberKind(precision, scale int64, ok bool) kind.Kind {
	if !ok {
		return kind.Decimal
	}
	if scale == 0 && precision > 0 && precision <= 19 {
		return kind.Int
	}
	return kind.Decimal
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
	fmt.Fprintf(&buf, "OFFSET %d ROWS", offset)

	if rr.Limit > -1 {
		fmt.Fprintf(&buf, " FETCH NEXT %d ROWS ONLY", rr.Limit)
	}

	return buf.String(), nil
}

// oracleTableAliasASRE matches `FROM|JOIN <tbl> AS <alias>` where both the
// table reference (optionally schema-qualified) and alias are double-quoted
// identifiers. Oracle rejects the AS keyword between a table reference and
// its alias; column aliases (e.g. `SELECT col AS alias`) are unaffected.
var oracleTableAliasASRE = regexp.MustCompile(
	`((?:FROM|JOIN)\s+(?:"[^"]+"\.)?"[^"]+")\s+AS(\s+"[^"]+")`,
)

// stripOracleTableAliasAS removes the AS keyword from table-alias positions
// in a rendered FROM/JOIN fragment. See oracleTableAliasASRE for matching
// rules.
func stripOracleTableAliasAS(s string) string {
	return oracleTableAliasASRE.ReplaceAllString(s, "$1$2")
}

// preRenderOracle adapts the rendered fragments for Oracle's dialect:
//   - Injects `FROM DUAL` when the query has no FROM clause; Oracle's
//     classic SQL grammar requires a row source for SELECT, and even on
//     Oracle 23ai a literal-only projection like `SELECT NULL` returns
//     zero rows via go-ora.
//   - Strips the AS keyword from table-alias positions in the FROM clause;
//     Oracle accepts `FROM tbl alias` but not `FROM tbl AS alias`.
//   - Ensures ORDER BY exists when a row range is used; Oracle requires
//     ORDER BY before OFFSET/FETCH (same pattern as SQL Server).
func preRenderOracle(_ *render.Context, f *render.Fragments) error {
	if f.From == "" {
		f.From = "FROM DUAL"
	} else {
		f.From = stripOracleTableAliasAS(f.From)
	}
	if f.Range != "" && f.OrderBy == "" {
		f.OrderBy = "ORDER BY (SELECT 0 FROM DUAL)"
	}
	return nil
}

// buildCreateTableStmt builds a CREATE TABLE statement for Oracle.
func buildCreateTableStmt(tblDef *schema.Table) string {
	sb := strings.Builder{}
	sb.WriteString(`CREATE TABLE `)
	sb.WriteString(enquoteOracle(tblDef.Name))
	sb.WriteString(` (`)

	for i, colDef := range tblDef.Cols {
		sb.WriteString("\n  ")
		sb.WriteString(enquoteOracle(colDef.Name))
		sb.WriteRune(' ')
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
