package duckdb_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/typetest"
)

// TestColumnTypes checks what sq reports for each DuckDB column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
//
// DuckDB is embedded, so this runs in every CI job, including under -short.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, typetest.Config{
		Handle:    sakila.Duck,
		RowIDDecl: "INTEGER NOT NULL PRIMARY KEY",
		Cases:     duckTypeCases,
	})
}

var (
	duckTimestampMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	duckDecimal5_2Max   = decimal.RequireFromString("999.99")
)

var duckTypeCases = []typetest.Case{
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "BOOLEAN", "BOOLEAN", kind.Bool),

	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxInt8), "TINYINT", "TINYINT", kind.Int),
	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(math.MaxInt16), "SMALLINT", "SMALLINT", kind.Int),
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, int64(math.MaxInt32), "INTEGER", "INTEGER", kind.Int),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "BIGINT", "BIGINT", kind.Int),
	typetest.NewCase("hugeint", "HUGEINT", typetest.SQL("0::HUGEINT"),
		typetest.SQL("170141183460469231731687303715884105727::HUGEINT"), "HUGEINT", "HUGEINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("170141183460469231731687303715884105727")),
	typetest.NewCase("utinyint", "UTINYINT", fixt.IntZ, int64(math.MaxUint8), "UTINYINT", "UTINYINT", kind.Int),
	typetest.NewCase("usmallint", "USMALLINT", fixt.IntZ, int64(math.MaxUint16), "USMALLINT", "USMALLINT",
		kind.Int),
	typetest.NewCase("uinteger", "UINTEGER", fixt.IntZ, int64(math.MaxUint32), "UINTEGER", "UINTEGER", kind.Int),

	// Go's database/sql can't bind a uint64 with the high bit set, so the
	// UBIGINT maximum goes in as a SQL expression.
	typetest.NewCase("ubigint", "UBIGINT", fixt.IntZ, typetest.SQL("18446744073709551615::UBIGINT"), "UBIGINT",
		"UBIGINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("18446744073709551615")),
	typetest.NewCase("uhugeint", "UHUGEINT", typetest.SQL("0::UHUGEINT"),
		typetest.SQL("340282366920938463463374607431768211455::UHUGEINT"), "UHUGEINT", "UHUGEINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("340282366920938463463374607431768211455")),

	// FLOAT is single precision.
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "FLOAT", "FLOAT", kind.Float).
		WithWantVal(float64(float32(fixt.Float))),
	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "DOUBLE", "DOUBLE", kind.Float),

	// DECIMAL is DECIMAL(18,3).
	typetest.NewCase("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "DECIMAL(18,3)", "DECIMAL(18,3)",
		kind.Decimal),
	typetest.NewCase("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, duckDecimal5_2Max, "DECIMAL(5,2)",
		"DECIMAL(5,2)", kind.Decimal),

	typetest.NewCase("varchar", "VARCHAR", fixt.TextZ, fixt.Text, "VARCHAR", "VARCHAR", kind.Text),

	// DuckDB discards a VARCHAR length, so VARCHAR(16) is plain VARCHAR.
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "VARCHAR", "VARCHAR", kind.Text),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "BLOB", "BLOB", kind.Bytes),
	typetest.NewCase("bit", "BIT", typetest.SQL("'0'::BIT"), typetest.SQL("'0101'::BIT"), "BIT", "BIT", kind.Text).
		WithWantZero("0").
		WithWantVal("0101"),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "DATE", "DATE", kind.Date),

	// TIME and TIMETZ values come back as a time.Time on 0001-01-01.
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "TIME", "TIME", kind.Time).
		WithWantZero(timeOfDay(0, 0, 0)).
		WithWantVal(timeOfDay(7, 7, 7)),
	typetest.NewCase("timetz", "TIMETZ", typetest.SQL("'00:00:00+00'::TIMETZ"), typetest.SQL("'07:07:07+00'::TIMETZ"),
		"TIME WITH TIME ZONE", "TIME WITH TIME ZONE", kind.Time).
		WithWantZero(timeOfDay(0, 0, 0)).
		WithWantVal(timeOfDay(7, 7, 7)),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP",
		kind.Datetime),
	typetest.NewCase("timestamp_ms", "TIMESTAMP_MS", fixt.DatetimeZ, duckTimestampMillis, "TIMESTAMP_MS",
		"TIMESTAMP_MS", kind.Datetime),
	typetest.NewCase("timestamp_s", "TIMESTAMP_S", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP_S", "TIMESTAMP_S",
		kind.Datetime),
	typetest.NewCase("timestamp_ns", "TIMESTAMP_NS", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP_NS",
		"TIMESTAMP_NS", kind.Datetime),
	typetest.NewCase("timestamptz", "TIMESTAMPTZ", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP WITH TIME ZONE",
		"TIMESTAMP WITH TIME ZONE", kind.Datetime),
	typetest.NewCase("interval", "INTERVAL", typetest.SQL("INTERVAL '0 seconds'"),
		typetest.SQL("INTERVAL '1 year 2 months 3 days 04:05:06.789'"), "INTERVAL", "INTERVAL", kind.Text).
		WithWantZero("00:00:00").
		WithWantVal("1 year 2 months 3 days 04:05:06.789"),

	// candidate issue: UUID is kind text, but its values come back as a
	// string of the raw 16 bytes, not the UUID's text form.
	typetest.NewCase("uuid", "UUID", "00000000-0000-0000-0000-000000000000", "77777777-7777-7777-7777-777777777777",
		"UUID", "UUID", kind.Text).
		WithWantZero(strings.Repeat("\x00", 16)).
		WithWantVal(strings.Repeat("w", 16)),

	// candidate issue: JSON values come back as the Go formatting of the
	// decoded value (map[val:7]), not as JSON text.
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Text).
		WithWantZero("map[]").
		WithWantVal("map[val:7]"),

	// An inline ENUM reports its values in its type name.
	typetest.NewCase("enum", "ENUM('alfa', 'bravo', 'charlie')", fixt.EnumAlfa, fixt.EnumBravo,
		"ENUM('alfa', 'bravo', 'charlie')", "ENUM('alfa', 'bravo', 'charlie')", kind.Text),

	// The composite types come back as the Go formatting of the decoded
	// value ([1 7], map[a:7 b:seven], OrderedMap{k: 7}) rather than as JSON:
	// #609. FLOAT[3], a fixed-size array, also has no kind in inspect: #1305.

	typetest.NewCase("integer_list", "INTEGER[]", typetest.SQL("[]::INTEGER[]"), typetest.SQL("[1, 7]"),
		"INTEGER[]", "INTEGER[]", kind.Text).
		WithWantZero("[]").
		WithWantVal("[1 7]").
		WithIssue("#609"),
	typetest.NewCase("float_array_3", "FLOAT[3]", typetest.SQL("[0, 0, 0]::FLOAT[3]"),
		typetest.SQL("[1, 2, 3]::FLOAT[3]"), "FLOAT[3]", "FLOAT[3]", kind.Unknown).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Text),
			Zero:      typetest.Is[any]("[0 0 0]"),
			Val:       typetest.Is[any]("[1 2 3]"),
		}).
		WithIssue("#1305"),
	typetest.NewCase("struct", "STRUCT(a INTEGER, b VARCHAR)", typetest.SQL("{'a': 0, 'b': ''}"),
		typetest.SQL("{'a': 7, 'b': 'seven'}"), "STRUCT(a INTEGER, b VARCHAR)", "STRUCT(a INTEGER, b VARCHAR)",
		kind.Text).
		WithWantZero("map[a:0 b:]").
		WithWantVal("map[a:7 b:seven]").
		WithIssue("#609"),
	typetest.NewCase("map", "MAP(VARCHAR, INTEGER)", typetest.SQL("MAP {}"), typetest.SQL("MAP {'k': 7}"),
		"MAP(VARCHAR, INTEGER)", "MAP(VARCHAR, INTEGER)", kind.Text).
		WithWantZero("OrderedMap{}").
		WithWantVal("OrderedMap{k: 7}").
		WithIssue("#609"),
}

// timeOfDay returns the time.Time that a TIME column holding the given time
// of day reads back as: that time on 0001-01-01 UTC.
func timeOfDay(hour, minute, sec int) time.Time {
	return time.Date(1, time.January, 1, hour, minute, sec, 0, time.UTC)
}
