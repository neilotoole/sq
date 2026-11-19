package sqlite3_test

import (
	"encoding/hex"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/typetest"
)

// TestColumnTypes checks what sq reports for each SQLite column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
//
// SQLite accepts any type name, and stores values by type affinity rather
// than by the declared type, so the cases include type names SQLite doesn't
// define. SQLite is embedded, so this runs in every CI job, including under
// -short.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, sl3Config)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "SQLite 3", typetest.DDLTable{Name: "coltypes", Config: sl3Config})
}

func TestSL3DefaultExpr(t *testing.T) {
	col := typetest.Column{Case: &typetest.Case{Decl: "INTEGER"}}
	for lit, want := range map[string]string{
		"7":                   "7",
		"-7":                  "-7",
		"7.7":                 "7.7",
		"'seven'":             "'seven'",
		"'it''s'":             "'it''s'",
		"X'7365'":             "X'7365'",
		"TRUE":                "TRUE",
		"'a' || 'b'":          "('a' || 'b')",
		"json_object('a', 1)": "(json_object('a', 1))",
	} {
		require.Equal(t, want, sl3DefaultExpr(col, lit), lit)
	}
}

func TestSL3Literal(t *testing.T) {
	col := typetest.Column{Case: &typetest.Case{Decl: "DATETIME"}}

	// The same text the sqlite3 driver stores for a bound time.Time.
	got, ok := sl3Literal(col, time.Date(2017, 7, 7, 0, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.Equal(t, "'2017-07-07 00:00:00+00:00'", got)
	got, ok = sl3Literal(col, time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.FixedZone("", 3600)))
	require.True(t, ok)
	require.Equal(t, "'2017-07-07 06:07:07.123+00:00'", got)
}

// sl3Config is the typetest configuration for SQLite.
var sl3Config = typetest.Config{
	Handle:      sakila.SL3,
	RowIDDecl:   "INTEGER NOT NULL PRIMARY KEY",
	Cases:       sl3TypeCases,
	Literal:     sl3Literal,
	DefaultExpr: sl3DefaultExpr,
}

// sl3DefaultExpr writes lit as the default expression of a Defaulted column:
// as is if it's a plain literal (a number, a quoted string, a blob, TRUE or
// FALSE), and otherwise in parentheses, since SQLite accepts an expression as
// a default only in parentheses.
func sl3DefaultExpr(_ typetest.Column, lit string) string {
	if sl3PlainLiteralRx.MatchString(lit) {
		return lit
	}
	return "(" + lit + ")"
}

// sl3PlainLiteralRx matches a SQLite literal that's valid as a default
// without parentheses.
var sl3PlainLiteralRx = regexp.MustCompile(
	`^(-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?|'([^']|'')*'|X'[0-9a-fA-F]*'|TRUE|FALSE)$`,
)

// sl3Literal renders the inputs SQLite writes differently from
// typetest.DefaultLiteral: bytes, as a blob literal (X'7365'), and a time as
// the same text the sqlite3 driver stores for a bound time.Time
// ('2017-07-07 07:07:07.123+00:00').
func sl3Literal(_ typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case []byte:
		if v != nil {
			return "X'" + hex.EncodeToString(v) + "'", true
		}
	case time.Time:
		return "'" + v.UTC().Format("2006-01-02 15:04:05.999999999-07:00") + "'", true
	}
	return "", false
}

var sl3TypeCases = []typetest.Case{
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, int64(math.MaxInt64), "INTEGER", "INTEGER", kind.Int).
		WithWantDefault("9223372036854775807"),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "INT", "INT", kind.Int).
		WithWantDefault("2147483647"),
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxInt8), "TINYINT", "TINYINT", kind.Int).
		WithWantDefault("127"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "BIGINT", "BIGINT", kind.Int).
		WithWantDefault("9223372036854775807"),
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "BOOLEAN", "BOOLEAN", kind.Bool).
		WithWantDefault("TRUE"),
	typetest.NewCase("real", "REAL", fixt.FloatZ, fixt.Float, "REAL", "REAL", kind.Float).
		WithWantDefault("7.7"),
	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "DOUBLE", "DOUBLE", kind.Float).
		WithWantDefault("7.7"),
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "FLOAT", "FLOAT", kind.Float).
		WithWantDefault("7.7"),
	typetest.NewCase("numeric", "NUMERIC", fixt.DecimalZ, fixt.Decimal, "NUMERIC", "NUMERIC", kind.Decimal).
		WithWantDefault("77.77"),
	typetest.NewCase("decimal_10_5", "DECIMAL(10,5)", fixt.DecimalZ, fixt.Decimal, "DECIMAL(10,5)",
		"DECIMAL(10,5)", kind.Decimal).
		WithWantDefault("77.77"),

	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "TEXT", "TEXT", kind.Text).
		WithWantDefault("'seven'"),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "VARCHAR(16)", "VARCHAR(16)",
		kind.Text).
		WithWantDefault("'0123456789abcdef'"),
	typetest.NewCase("char_10", "CHAR(10)", fixt.TextZ, fixt.Text, "CHAR(10)", "CHAR(10)", kind.Text).
		WithWantDefault("'seven'"),
	typetest.NewCase("clob", "CLOB", fixt.TextZ, fixt.Text, "CLOB", "CLOB", kind.Text).
		WithWantDefault("'seven'"),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "BLOB", "BLOB", kind.Bytes).
		WithWantDefault("X'736576656e'"),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "DATE", "DATE", kind.Date).
		WithWantDefault("'2017-07-07 00:00:00+00:00'"),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "DATETIME", "DATETIME", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07+00:00'"),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP",
		kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07+00:00'"),
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "TIME", "TIME", kind.Time).
		WithWantDefault("'07:07:07'"),

	// candidate issue: JSON has no kind in inspect, but its query result kind
	// is text.
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Unknown).
		WithWant(typetest.Want{QueryKind: typetest.Is(kind.Text)}).
		WithWantDefault("'{\"val\": 7}'"),

	// A column declared with no type, and one with a type name SQLite doesn't
	// define. Both take any value: here an integer in row 1 and the text
	// "seven" in rows 2 and 3.
	//
	// candidate issue: the untyped column's base_type and column_type are
	// "integer", taken from its first stored value, though it has no declared
	// type; its kind is int, yet a text value reads back as text. WIDGET has
	// no kind in inspect, but its query result kind is int, taken from the
	// first value.
	//
	// The Defaulted columns hold the text default 'seven' in every row, so
	// for them that first value makes the type text instead.
	typetest.NewCase("untyped", "", fixt.IntZ, fixt.Text, "integer", "integer", kind.Int).
		WithWantDefault("'seven'").
		WithPerVariant(typetest.PerVariant{Defaulted: typetest.Want{
			BaseType: typetest.Is("text"), ColumnType: typetest.Is("text"), Kind: typetest.Is(kind.Text),
		}}),
	typetest.NewCase("widget", "WIDGET", fixt.IntZ, fixt.Text, "WIDGET", "WIDGET", kind.Unknown).
		WithWant(typetest.Want{QueryKind: typetest.Is(kind.Int)}).
		WithWantDefault("'seven'").
		WithPerVariant(typetest.PerVariant{Defaulted: typetest.Want{QueryKind: typetest.Is(kind.Text)}}),

	// A value that doesn't match the declared type: SQLite stores the text
	// "seven" in an INTEGER column as text, and querying the column then
	// fails, since the scan type comes from the declared type.
	typetest.NewCase("integer_affinity", "INTEGER", fixt.IntZ, fixt.Text, "INTEGER", "INTEGER", kind.Int).
		WithWant(typetest.Want{QueryErr: typetest.Is(`converting driver.Value type string ("seven") to a int64`)}).
		WithIssue("#1299").
		WithWantDefault("'seven'"),
}
