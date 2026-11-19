package duckdb_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

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
	typetest.Run(t, duckConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "DuckDB 2.0",
		typetest.DDLTable{Name: "coltypes", Config: duckConfig})
}

func TestDuckLiteral(t *testing.T) {
	got, ok := duckLiteral(typetest.Column{Case: &typetest.Case{Decl: "TIMESTAMP WITH TIME ZONE"}},
		time.Date(2017, 7, 7, 7, 7, 7, 0, time.UTC))
	require.True(t, ok)
	require.Equal(t, "'2017-07-07 07:07:07+00:00'", got)
}

// duckConfig is the typetest configuration for DuckDB.
var duckConfig = typetest.Config{
	Handle:    sakila.Duck,
	RowIDDecl: "INTEGER NOT NULL PRIMARY KEY",
	Cases:     duckTypeCases,
	Literal:   duckLiteral,
	// Load the generated DDL in an odd time zone, so a time literal without
	// an offset fails the check. See typetest.Config.DDLCheckSession.
	DDLCheckSession: []string{"SET SESSION TimeZone = '" + typetest.CheckTimeZone + "'"},
}

// duckLiteral renders the inputs DuckDB writes differently from
// typetest.DefaultLiteral: bytes, as a BLOB literal with each byte escaped
// ('\x73\x65'::BLOB), and a TIMESTAMPTZ value with an explicit offset, since
// DuckDB reads one without an offset in the session's time zone.
func duckLiteral(col typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case []byte:
		if v == nil {
			return "", false
		}
		var sb strings.Builder
		sb.WriteString("'")
		for _, c := range v {
			fmt.Fprintf(&sb, "\\x%02x", c)
		}
		sb.WriteString("'::BLOB")
		return sb.String(), true
	case time.Time:
		if typetest.HasTimeZone(col.Case.Decl) {
			return typetest.TimestampTZLiteral(v), true
		}
	}
	return "", false
}

var (
	duckTimestampMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	duckDecimal5_2Max   = decimal.RequireFromString("999.99")
)

var duckTypeCases = []typetest.Case{
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "BOOLEAN", "BOOLEAN", kind.Bool).
		WithWantDefault("true"),

	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxInt8), "TINYINT", "TINYINT", kind.Int).
		WithWantDefault("127"),
	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(math.MaxInt16), "SMALLINT", "SMALLINT", kind.Int).
		WithWantDefault("32767"),
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, int64(math.MaxInt32), "INTEGER", "INTEGER", kind.Int).
		WithWantDefault("2147483647"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "BIGINT", "BIGINT", kind.Int).
		WithWantDefault("9223372036854775807"),
	typetest.NewCase("hugeint", "HUGEINT", typetest.SQL("0::HUGEINT"),
		typetest.SQL("170141183460469231731687303715884105727::HUGEINT"), "HUGEINT", "HUGEINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("170141183460469231731687303715884105727")).
		WithWantDefault("CAST(170141183460469231731687303715884105727 AS HUGEINT)"),
	typetest.NewCase("utinyint", "UTINYINT", fixt.IntZ, int64(math.MaxUint8), "UTINYINT", "UTINYINT", kind.Int).
		WithWantDefault("255"),
	typetest.NewCase("usmallint", "USMALLINT", fixt.IntZ, int64(math.MaxUint16), "USMALLINT", "USMALLINT",
		kind.Int).
		WithWantDefault("65535"),
	typetest.NewCase("uinteger", "UINTEGER", fixt.IntZ, int64(math.MaxUint32), "UINTEGER", "UINTEGER", kind.Int).
		WithWantDefault("4294967295"),

	// Go's database/sql can't bind a uint64 with the high bit set, so the
	// UBIGINT maximum goes in as a SQL expression.
	typetest.NewCase("ubigint", "UBIGINT", fixt.IntZ, typetest.SQL("18446744073709551615::UBIGINT"), "UBIGINT",
		"UBIGINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("18446744073709551615")).
		WithWantDefault("CAST(18446744073709551615 AS UBIGINT)"),
	typetest.NewCase("uhugeint", "UHUGEINT", typetest.SQL("0::UHUGEINT"),
		typetest.SQL("340282366920938463463374607431768211455::UHUGEINT"), "UHUGEINT", "UHUGEINT", kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.RequireFromString("340282366920938463463374607431768211455")).
		WithWantDefault("CAST(340282366920938463463374607431768211455 AS UHUGEINT)"),

	// FLOAT is single precision.
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "FLOAT", "FLOAT", kind.Float).
		WithWantVal(float64(float32(fixt.Float))).
		WithWantDefault("7.7"),
	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "DOUBLE", "DOUBLE", kind.Float).
		WithWantDefault("7.7"),

	// DECIMAL is DECIMAL(18,3).
	typetest.NewCase("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "DECIMAL(18,3)", "DECIMAL(18,3)",
		kind.Decimal).
		WithWantDefault("77.77"),
	typetest.NewCase("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, duckDecimal5_2Max, "DECIMAL(5,2)",
		"DECIMAL(5,2)", kind.Decimal).
		WithWantDefault("999.99"),

	typetest.NewCase("varchar", "VARCHAR", fixt.TextZ, fixt.Text, "VARCHAR", "VARCHAR", kind.Text).
		WithWantDefault("'seven'"),

	// DuckDB discards a VARCHAR length, so VARCHAR(16) is plain VARCHAR.
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "VARCHAR", "VARCHAR", kind.Text).
		WithWantDefault("'0123456789abcdef'"),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "BLOB", "BLOB", kind.Bytes).
		WithWantDefault("CAST('\\x73\\x65\\x76\\x65\\x6e' AS BLOB)"),
	typetest.NewCase("bit", "BIT", typetest.SQL("'0'::BIT"), typetest.SQL("'0101'::BIT"), "BIT", "BIT", kind.Text).
		WithWantZero("0").
		WithWantVal("0101").
		WithWantDefault("CAST('0101' AS BIT)"),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "DATE", "DATE", kind.Date).
		WithWantDefault("'2017-07-07'"),

	// TIME and TIMETZ values come back as a time.Time on 0001-01-01.
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "TIME", "TIME", kind.Time).
		WithWantZero(typetest.TimeOfDay(0, 0, 0, 0)).
		WithWantVal(typetest.TimeOfDay(7, 7, 7, 0)).
		WithWantDefault("'07:07:07'"),
	typetest.NewCase("timetz", "TIMETZ", typetest.SQL("'00:00:00+00'::TIMETZ"), typetest.SQL("'07:07:07+00'::TIMETZ"),
		"TIME WITH TIME ZONE", "TIME WITH TIME ZONE", kind.Time).
		WithWantZero(typetest.TimeOfDay(0, 0, 0, 0)).
		WithWantVal(typetest.TimeOfDay(7, 7, 7, 0)).
		WithWantDefault("CAST('07:07:07+00' AS TIMETZ)"),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP",
		kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'"),
	typetest.NewCase("timestamp_ms", "TIMESTAMP_MS", fixt.DatetimeZ, duckTimestampMillis, "TIMESTAMP_MS",
		"TIMESTAMP_MS", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07.123'"),
	typetest.NewCase("timestamp_s", "TIMESTAMP_S", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP_S", "TIMESTAMP_S",
		kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'"),
	typetest.NewCase("timestamp_ns", "TIMESTAMP_NS", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP_NS",
		"TIMESTAMP_NS", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'"),
	typetest.NewCase("timestamptz", "TIMESTAMPTZ", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP WITH TIME ZONE",
		"TIMESTAMP WITH TIME ZONE", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07+00:00'"),
	typetest.NewCase("interval", "INTERVAL", typetest.SQL("INTERVAL '0 seconds'"),
		typetest.SQL("INTERVAL '1 year 2 months 3 days 04:05:06.789'"), "INTERVAL", "INTERVAL", kind.Text).
		WithWantZero("00:00:00").
		WithWantVal("1 year 2 months 3 days 04:05:06.789").
		WithWantDefault("CAST('1 year 2 months 3 days 04:05:06.789' AS INTERVAL)"),

	// candidate issue: UUID is kind text, but its values come back as a
	// string of the raw 16 bytes, not the UUID's text form.
	typetest.NewCase("uuid", "UUID", "00000000-0000-0000-0000-000000000000", "77777777-7777-7777-7777-777777777777",
		"UUID", "UUID", kind.Text).
		WithWantZero(strings.Repeat("\x00", 16)).
		WithWantVal(strings.Repeat("w", 16)).
		WithWantDefault("'77777777-7777-7777-7777-777777777777'"),

	// candidate issue: JSON values come back as the Go formatting of the
	// decoded value (map[val:7]), not as JSON text.
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Text).
		WithWantZero("map[]").
		WithWantVal("map[val:7]").
		WithWantDefault("'{\"val\": 7}'"),

	// An inline ENUM reports its values in its type name.
	typetest.NewCase("enum", "ENUM('alfa', 'bravo', 'charlie')", fixt.EnumAlfa, fixt.EnumBravo,
		"ENUM('alfa', 'bravo', 'charlie')", "ENUM('alfa', 'bravo', 'charlie')", kind.Text).
		WithWantDefault("'bravo'"),

	// The composite types come back as the Go formatting of the decoded
	// value ([1 7], map[a:7 b:seven], OrderedMap{k: 7}) rather than as JSON:
	// #609. FLOAT[3], a fixed-size array, also has no kind in inspect: #1305.

	typetest.NewCase("integer_list", "INTEGER[]", typetest.SQL("[]::INTEGER[]"), typetest.SQL("[1, 7]"),
		"INTEGER[]", "INTEGER[]", kind.Text).
		WithWantZero("[]").
		WithWantVal("[1 7]").
		WithIssue("#609").
		WithWantDefault("list_value(1, 7)"),
	typetest.NewCase("float_array_3", "FLOAT[3]", typetest.SQL("[0, 0, 0]::FLOAT[3]"),
		typetest.SQL("[1, 2, 3]::FLOAT[3]"), "FLOAT[3]", "FLOAT[3]", kind.Unknown).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Text),
			Zero:      typetest.Is[any]("[0 0 0]"),
			Val:       typetest.Is[any]("[1 2 3]"),
		}).
		WithIssue("#1305").
		WithWantDefault("CAST(list_value(1, 2, 3) AS FLOAT[3])"),
	typetest.NewCase("struct", "STRUCT(a INTEGER, b VARCHAR)", typetest.SQL("{'a': 0, 'b': ''}"),
		typetest.SQL("{'a': 7, 'b': 'seven'}"), "STRUCT(a INTEGER, b VARCHAR)", "STRUCT(a INTEGER, b VARCHAR)",
		kind.Text).
		WithWantZero("map[a:0 b:]").
		WithWantVal("map[a:7 b:seven]").
		WithIssue("#609").
		WithWantDefault("struct_pack(a := 7, b := 'seven')"),
	typetest.NewCase("map", "MAP(VARCHAR, INTEGER)", typetest.SQL("MAP {}"), typetest.SQL("MAP {'k': 7}"),
		"MAP(VARCHAR, INTEGER)", "MAP(VARCHAR, INTEGER)", kind.Text).
		WithWantZero("OrderedMap{}").
		WithWantVal("OrderedMap{k: 7}").
		WithIssue("#609").
		WithWantDefault("\"map\"(list_value('k'), list_value(7))"),
}
