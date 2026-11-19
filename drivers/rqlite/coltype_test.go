package rqlite_test

import (
	"math"
	"testing"

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

	typetest.Run(t, typetest.Config{
		Handle:    sakila.RQ,
		RowIDDecl: "INTEGER NOT NULL PRIMARY KEY",
		Cases:     rqTypeCases,
	})
}

// rqFirstInexactInt is 2^53 + 1, the smallest integer a float64 can't hold
// exactly: it reads back as 2^53. See the INTEGER case.
const rqFirstInexactInt = int64(1<<53 + 1)

var rqTypeCases = []typetest.Case{
	// candidate issue: rqlite's HTTP API returns integers as JSON numbers,
	// which the driver decodes through float64, so integers beyond 2^53 lose
	// precision: 2^53 + 1 reads back as 2^53. The cases use that value rather
	// than math.MaxInt64, because math.MaxInt64 rounds to 2^63, which is out
	// of int64 range, and Go's conversion of an out-of-range float to int64 is
	// implementation-dependent: it reads back as math.MinInt64 on amd64 and
	// as math.MaxInt64 on arm64.
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, rqFirstInexactInt, "INTEGER", "INTEGER", kind.Int).
		WithWantVal(int64(1 << 53)),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "INT", "INT", kind.Int),
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxInt8), "TINYINT", "TINYINT", kind.Int),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, rqFirstInexactInt, "BIGINT", "BIGINT", kind.Int).
		WithWantVal(int64(1 << 53)),
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "BOOLEAN", "BOOLEAN", kind.Bool),
	typetest.NewCase("real", "REAL", fixt.FloatZ, fixt.Float, "REAL", "REAL", kind.Float),
	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "DOUBLE", "DOUBLE", kind.Float),
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "FLOAT", "FLOAT", kind.Float),
	typetest.NewCase("numeric", "NUMERIC", fixt.DecimalZ, fixt.Decimal, "NUMERIC", "NUMERIC", kind.Decimal),
	typetest.NewCase("decimal_10_5", "DECIMAL(10,5)", fixt.DecimalZ, fixt.Decimal, "DECIMAL(10,5)",
		"DECIMAL(10,5)", kind.Decimal),

	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "TEXT", "TEXT", kind.Text),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "VARCHAR(16)", "VARCHAR(16)",
		kind.Text),
	typetest.NewCase("char_10", "CHAR(10)", fixt.TextZ, fixt.Text, "CHAR(10)", "CHAR(10)", kind.Text),
	typetest.NewCase("clob", "CLOB", fixt.TextZ, fixt.Text, "CLOB", "CLOB", kind.Text),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "BLOB", "BLOB", kind.Bytes),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "DATE", "DATE", kind.Date),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "DATETIME", "DATETIME", kind.Datetime),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP",
		kind.Datetime),
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "TIME", "TIME", kind.Time),

	// candidate issue: JSON has no kind in inspect, but its query result kind
	// is text.
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Unknown).
		WithWant(typetest.Want{QueryKind: typetest.Is(kind.Text)}),

	// A column declared with no type, and one with a type name SQLite doesn't
	// define. Both take any value: here an integer in row 1 and the text
	// "seven" in rows 2 and 3.
	//
	// candidate issue: unlike the sqlite3 driver, which infers an untyped
	// column's type from its first value, rqlite reports no type and kind
	// bytes, and querying the column fails. WIDGET has no kind in inspect,
	// but its query result kind is float (int on SQLite), and the integer
	// reads back as a float.
	typetest.NewCase("untyped", "", fixt.IntZ, fixt.Text, "", "", kind.Bytes).
		WithWant(typetest.Want{QueryErr: typetest.Is(`converting driver.Value type string ("seven") to a int64`)}),
	typetest.NewCase("widget", "WIDGET", fixt.IntZ, fixt.Text, "WIDGET", "WIDGET", kind.Unknown).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Float),
			Zero:      typetest.Is[any](float64(0)),
		}),

	// A value that doesn't match the declared type: SQLite stores the text
	// "seven" in an INTEGER column as text, and querying the column then
	// fails, since the scan type comes from the declared type.
	typetest.NewCase("integer_affinity", "INTEGER", fixt.IntZ, fixt.Text, "INTEGER", "INTEGER", kind.Int).
		WithWant(typetest.Want{QueryErr: typetest.Is(`converting driver.Value type string ("seven") to a int64`)}).
		WithIssue("#1299"),
}
