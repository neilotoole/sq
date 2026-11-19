package rqlite_test

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

// TestColumnTypes checks what sq reports for each rqlite column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
//
// rqlite is SQLite behind an HTTP API, so the cases are those of the sqlite3
// driver's TestColumnTypes; the expectations differ where rqlite's API or
// sq's rqlite driver does.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, rqConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "rqlite 10", typetest.DDLTable{Name: "coltypes", Config: rqConfig})
}

func TestRQDefaultExpr(t *testing.T) {
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
		require.Equal(t, want, rqDefaultExpr(col, lit), lit)
	}
}

func TestRQLiteral(t *testing.T) {
	col := typetest.Column{Case: &typetest.Case{Decl: "DATETIME"}}

	// The same text rqlite stores for a bound time.Time.
	got, ok := rqLiteral(col, time.Date(2017, 7, 7, 0, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.Equal(t, "'2017-07-07T00:00:00Z'", got)
	got, ok = rqLiteral(col, time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.FixedZone("", 3600)))
	require.True(t, ok)
	require.Equal(t, "'2017-07-07T06:07:07.123Z'", got)
}

// rqConfig is the typetest configuration for rqlite.
var rqConfig = typetest.Config{
	Handle:      sakila.RQ,
	RowIDDecl:   "INTEGER NOT NULL PRIMARY KEY",
	Cases:       rqTypeCases,
	Literal:     rqLiteral,
	DefaultExpr: rqDefaultExpr,
}

// rqDefaultExpr writes lit as the default expression of a Defaulted column:
// as is if it's a plain literal (a number, a quoted string, a blob, TRUE or
// FALSE), and otherwise in parentheses, since SQLite, under rqlite, accepts an
// expression as a default only in parentheses.
func rqDefaultExpr(_ typetest.Column, lit string) string {
	if rqPlainLiteralRx.MatchString(lit) {
		return lit
	}
	return "(" + lit + ")"
}

// rqPlainLiteralRx matches a SQLite literal that's valid as a default without
// parentheses.
var rqPlainLiteralRx = regexp.MustCompile(
	`^(-?[0-9]+(\.[0-9]+)?([eE][-+]?[0-9]+)?|'([^']|'')*'|X'[0-9a-fA-F]*'|TRUE|FALSE)$`,
)

// rqLiteral renders the inputs rqlite writes differently from
// typetest.DefaultLiteral: bytes, as a blob literal (X'7365'), and a time as
// the same text rqlite stores for a bound time.Time, RFC 3339 in UTC
// ('2017-07-07T07:07:07.123Z').
func rqLiteral(_ typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case []byte:
		if v != nil {
			return "X'" + hex.EncodeToString(v) + "'", true
		}
	case time.Time:
		return "'" + v.UTC().Format(time.RFC3339Nano) + "'", true
	}
	return "", false
}

// rqFirstInexactInt is 2^53 + 1, the smallest integer a float64 can't hold
// exactly: it reads back as 2^53. See the INTEGER case.
const rqFirstInexactInt = int64(1<<53 + 1)

// untypedQueryErr is the error querying the untyped column fails with, on
// the variants whose first stored value is an integer.
const untypedQueryErr = `converting driver.Value type string ("seven") to a int64`

var rqTypeCases = []typetest.Case{
	// #842 (item 6): rqlite's HTTP API returns integers as JSON numbers,
	// which the driver decodes through float64, so integers beyond 2^53 lose
	// precision: 2^53 + 1 reads back as 2^53. The cases use that value rather
	// than math.MaxInt64, because math.MaxInt64 rounds to 2^63, which is out
	// of int64 range, and Go's conversion of an out-of-range float to int64 is
	// implementation-dependent: it reads back as math.MinInt64 on amd64 and
	// as math.MaxInt64 on arm64.
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, rqFirstInexactInt, "INTEGER", "INTEGER", kind.Int).
		WithWantVal(int64(1 << 53)).
		WithWantDefault("9007199254740993"),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "INT", "INT", kind.Int).
		WithWantDefault("2147483647"),
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxInt8), "TINYINT", "TINYINT", kind.Int).
		WithWantDefault("127"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, rqFirstInexactInt, "BIGINT", "BIGINT", kind.Int).
		WithWantVal(int64(1 << 53)).
		WithWantDefault("9007199254740993"),
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
		WithWantDefault("'2017-07-07T00:00:00Z'"),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "DATETIME", "DATETIME", kind.Datetime).
		WithWantDefault("'2017-07-07T07:07:07Z'"),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP",
		kind.Datetime).
		WithWantDefault("'2017-07-07T07:07:07Z'"),
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "TIME", "TIME", kind.Time).
		WithWantDefault("'07:07:07'"),

	// candidate issue (#1388): JSON has no kind in inspect, but its query result kind
	// is text.
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Unknown).
		WithWant(typetest.Want{QueryKind: typetest.Is(kind.Text)}).
		WithWantDefault("'{\"val\": 7}'"),

	// A column declared with no type, and one with a type name SQLite doesn't
	// define. Both take any value: here an integer in row 1 and the text
	// "seven" in rows 2 and 3.
	//
	// candidate issue (#1388): unlike the sqlite3 driver, which infers an untyped
	// column's type from its first value, rqlite reports no type and kind
	// bytes, and querying the column fails. WIDGET has no kind in inspect,
	// but its query result kind is float (int on SQLite), and the integer
	// reads back as a float.
	//
	// The Defaulted columns hold the text default 'seven' in every row, so for
	// them the query result kind is text, and the untyped column queries
	// without error.
	typetest.NewCase("untyped", "", fixt.IntZ, fixt.Text, "", "", kind.Bytes).
		WithPerVariant(typetest.PerVariant{
			NotNull:   typetest.Want{QueryErr: typetest.Is(untypedQueryErr)},
			Nullable:  typetest.Want{QueryErr: typetest.Is(untypedQueryErr)},
			Defaulted: typetest.Want{QueryKind: typetest.Is(kind.Text)},
		}).
		WithWantDefault("'seven'"),
	typetest.NewCase("widget", "WIDGET", fixt.IntZ, fixt.Text, "WIDGET", "WIDGET", kind.Unknown).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Float),
			Zero:      typetest.Is[any](float64(0)),
		}).
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
