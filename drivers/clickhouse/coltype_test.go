package clickhouse_test

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/typetest"
)

// TestColumnTypes checks what sq reports for each ClickHouse column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
//
// TestTypeMapping and its siblings cover the other direction: the ClickHouse
// type sq creates for each kind.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, chConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "ClickHouse 26", typetest.DDLTable{Name: "coltypes", Config: chConfig})
}

func TestCHLiteral(t *testing.T) {
	// ClickHouse reads a backslash in a string literal as an escape.
	got, ok := chLiteral(typetest.Column{Case: &typetest.Case{Decl: "String"}}, `a\nb'c`)
	require.True(t, ok)
	require.Equal(t, `'a\\nb''c'`, got)
}

// chConfig is the typetest configuration for ClickHouse.
var chConfig = typetest.Config{
	Handle:        sakila.CH,
	RowIDDecl:     "Int32",
	ColumnDecl:    chColumnDecl,
	TableSuffix:   "ENGINE = MergeTree() ORDER BY row_id",
	Cases:         withNullableColumnTypes(chTypeCases),
	CheckSettings: checkCHSettings,
	Literal:       chLiteral,
	// ClickHouse reads a DateTime literal in the session's time zone, and
	// the literals are UTC.
	DDLPrelude: []string{"SET session_timezone = 'UTC'"},
	// Load the generated DDL in an odd time zone, so a time literal that
	// depends on it fails the check. See typetest.Config.DDLCheckSession.
	DDLCheckSession: []string{"SET session_timezone = '" + typetest.CheckTimeZone + "'"},
}

// chLiteral renders the inputs ClickHouse writes differently from
// typetest.DefaultLiteral: a DateTime or DateTime64 value as a full date and
// time, which ClickHouse requires for those types even at midnight; and
// decimals and floats as quoted strings. ClickHouse parses a quoted number
// exactly as the column's type. Unquoted, 9999999.99 is read as a float and
// overflows Decimal(9, 2), and once a comment in the VALUES list sends
// ClickHouse to its expression parser, 7.7 reads back from a Nullable(Float32)
// column as NULL. Strings have backslashes escaped, since ClickHouse reads a
// backslash in a string literal as an escape.
func chLiteral(col typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case time.Time:
		if strings.HasPrefix(col.Case.Decl, "DateTime") {
			return "'" + v.UTC().Format("2006-01-02 15:04:05.999999999") + "'", true
		}
	case decimal.Decimal:
		return "'" + v.String() + "'", true
	case float64:
		return "'" + strconv.FormatFloat(v, 'g', -1, 64) + "'", true
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), "'", "''") + "'", true
	}
	return "", false
}

// checkCHSettings verifies that the server's time zone is UTC, as the
// expectations assume: clickhouse-go returns a DateTime column declared
// without a time zone in the server's time zone, and the comparison is
// sensitive to the time.Time's location.
func checkCHSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var tz string
	if err := db.QueryRowContext(ctx, "SELECT timezone()").Scan(&tz); err != nil {
		return errz.Err(err)
	}
	if tz != "UTC" {
		return errz.Errorf("ClickHouse %s: server time zone is {%s}, expected {UTC}", serverVer, tz)
	}
	return nil
}

// chColumnDecl declares a column the ClickHouse way: columns are NOT NULL by
// default, and a nullable column wraps its type in Nullable(T). For
// LowCardinality(T), which can't be wrapped, the nullable form wraps the inner
// type instead: LowCardinality(Nullable(T)).
func chColumnDecl(decl string, v typetest.Variant, defaultExpr string) string {
	switch v {
	case typetest.Defaulted:
		return decl + " DEFAULT " + defaultExpr
	case typetest.Nullable:
		if inner, ok := strings.CutPrefix(decl, "LowCardinality("); ok {
			return "LowCardinality(Nullable(" + inner + ")"
		}
		return "Nullable(" + decl + ")"
	case typetest.NotNull:
		return decl
	default:
		return decl
	}
}

// withNullableColumnTypes returns cases with the expected column_type of each
// nullable column set to Nullable(T), T being the case's column_type, which is
// how ClickHouse reports a nullable column. A case that sets its nullable
// column_type itself is left alone.
func withNullableColumnTypes(cases []typetest.Case) []typetest.Case {
	out := make([]typetest.Case, len(cases))
	for i, c := range cases {
		out[i] = c
		if c.PerVariant.Nullable.ColumnType.Set || !c.Creates(typetest.Nullable) {
			continue
		}
		out[i] = c.WithPerVariant(typetest.PerVariant{Nullable: typetest.Want{
			ColumnType: typetest.Is("Nullable(" + c.Want.ColumnType.Val + ")"),
		}})
	}
	return out
}

// chNetIPScanErr is the error from querying an IPv4 or IPv6 column.
const chNetIPScanErr = "unsupported Scan, storing driver.Value type net.IP into type *string"

var (
	chDatetimeMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	chDecimal9_2Max  = decimal.RequireFromString("9999999.99")
)

var chTypeCases = []typetest.Case{
	typetest.NewCase("int8", "Int8", fixt.IntZ, int64(math.MaxInt8), "Int8", "Int8", kind.Int).
		WithWantDefault("127"),
	typetest.NewCase("int16", "Int16", fixt.IntZ, int64(math.MaxInt16), "Int16", "Int16", kind.Int).
		WithWantDefault("32767"),
	typetest.NewCase("int32", "Int32", fixt.IntZ, int64(math.MaxInt32), "Int32", "Int32", kind.Int).
		WithWantDefault("2147483647"),
	typetest.NewCase("int64", "Int64", fixt.IntZ, int64(math.MaxInt64), "Int64", "Int64", kind.Int).
		WithWantDefault("9223372036854775807"),
	typetest.NewCase("uint8", "UInt8", fixt.IntZ, int64(math.MaxUint8), "UInt8", "UInt8", kind.Int).
		WithWantDefault("255"),
	typetest.NewCase("uint16", "UInt16", fixt.IntZ, int64(math.MaxUint16), "UInt16", "UInt16", kind.Int).
		WithWantDefault("65535"),
	typetest.NewCase("uint32", "UInt32", fixt.IntZ, int64(math.MaxUint32), "UInt32", "UInt32", kind.Int).
		WithWantDefault("4294967295"),

	// candidate issue (#1388): querying a UInt64 column holding a value above
	// math.MaxInt64 fails, since sq scans UInt64 into int64.
	typetest.NewCase("uint64", "UInt64", fixt.IntZ, typetest.SQL("toUInt64('18446744073709551615')"), "UInt64",
		"UInt64", kind.Int).
		WithWant(typetest.Want{
			QueryErr: typetest.Is(`converting driver.Value type uint64 ("18446744073709551615") to a int64`),
			Val:      typetest.WantNull(),
		}).
		WithWantDefault("toUInt64('18446744073709551615')"),

	// candidate issue (#1388): Int128 is kind text in inspect, and querying it fails:
	// the driver returns a big.Int, which sq can't scan.
	typetest.NewCase("int128", "Int128", fixt.IntZ, typetest.SQL("toInt128('170141183460469231731687303715884105727')"),
		"Int128", "Int128", kind.Text).
		WithWant(typetest.Want{
			QueryErr: typetest.Is("unsupported Scan, storing driver.Value type big.Int into type *string"),
			Val:      typetest.WantNull(),
		}).
		WithWantDefault("toInt128('170141183460469231731687303715884105727')"),

	typetest.NewCase("float32", "Float32", fixt.FloatZ, fixt.Float, "Float32", "Float32", kind.Float).
		WithWantDefault("'7.7'"),
	typetest.NewCase("float64", "Float64", fixt.FloatZ, fixt.Float, "Float64", "Float64", kind.Float).
		WithWantDefault("'7.7'"),

	// candidate issue (#1388): the NOT NULL Decimal columns' values (the NotNull and
	// Defaulted variants) come back as strings, while the nullable column's
	// come back as decimals.
	typetest.NewCase("decimal_9_2", "Decimal(9, 2)", fixt.DecimalZ, chDecimal9_2Max, "Decimal(9, 2)",
		"Decimal(9, 2)", kind.Decimal).
		WithPerVariant(typetest.PerVariant{
			NotNull: typetest.Want{
				Zero: typetest.Is[any]("0"),
				Val:  typetest.Is[any]("9999999.99"),
			},
			Defaulted: typetest.Want{Val: typetest.Is[any]("9999999.99")},
		}).
		WithWantVal(decimal.New(999999999, -2)).
		WithWantDefault("'9999999.99'"),
	typetest.NewCase("bool", "Bool", fixt.BoolZ, fixt.Bool, "Bool", "Bool", kind.Bool).
		WithWantDefault("true"),

	typetest.NewCase("string", "String", fixt.TextZ, fixt.Text, "String", "String", kind.Text).
		WithWantDefault("'seven'"),

	// FixedString pads values to its length with zero bytes.
	typetest.NewCase("fixed_string_16", "FixedString(16)", fixt.TextZ, "0123456789abcdef", "FixedString(16)",
		"FixedString(16)", kind.Text).
		WithWantZero(strings.Repeat("\x00", 16)).
		WithWantDefault("'0123456789abcdef'"),
	typetest.NewCase("uuid", "UUID", "00000000-0000-0000-0000-000000000000", "77777777-7777-7777-7777-777777777777",
		"UUID", "UUID", kind.Text).
		WithWantDefault("'77777777-7777-7777-7777-777777777777'"),
	typetest.NewCase("enum8", "Enum8('alfa' = 1, 'bravo' = 2, 'charlie' = 3)", fixt.EnumAlfa, fixt.EnumBravo,
		"Enum8('alfa' = 1, 'bravo' = 2, 'charlie' = 3)", "Enum8('alfa' = 1, 'bravo' = 2, 'charlie' = 3)",
		kind.Text).
		WithWantDefault("'bravo'"),

	// LowCardinality(T) can't be wrapped in Nullable, so its nullable column
	// is LowCardinality(Nullable(T)); see chColumnDecl.
	//
	// candidate issue (#1388): base_type is the wrapped type (String), while for
	// other parameterized types it keeps the parameters (Decimal(9, 2),
	// FixedString(16)).
	//
	// candidate issue (#1388): the nullable LowCardinality(Nullable(String)) column
	// is reported as not nullable, since the driver only recognizes a
	// leading Nullable(, and querying it fails: the driver returns a *string,
	// which sq can't scan.
	typetest.NewCase("low_cardinality_string", "LowCardinality(String)", fixt.TextZ, fixt.Text, "String",
		"LowCardinality(String)", kind.Text).
		WithPerVariant(typetest.PerVariant{Nullable: typetest.Want{
			ColumnType: typetest.Is("LowCardinality(Nullable(String))"),
			Nullable:   typetest.Is(false),
			QueryErr:   typetest.Is("unsupported Scan, storing driver.Value type *string into type *string"),
		}}).
		WithWantDefault("'seven'"),

	typetest.NewCase("date", "Date", fixt.DateZ, fixt.Date, "Date", "Date", kind.Date).
		WithWantDefault("'2017-07-07'"),
	typetest.NewCase("date32", "Date32", fixt.DateZ, fixt.Date, "Date32", "Date32", kind.Date).
		WithWantDefault("'2017-07-07'"),
	typetest.NewCase("datetime", "DateTime", fixt.DatetimeZ, fixt.Datetime, "DateTime", "DateTime", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'"),
	typetest.NewCase("datetime_utc", "DateTime('UTC')", fixt.DatetimeZ, fixt.Datetime, "DateTime('UTC')",
		"DateTime('UTC')", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'"),

	// candidate issue (#1388): the milliseconds of a bound DateTime64(3) value are
	// lost on insert: 07:07:07.123 reads back as 07:07:07. Loaded from the
	// DDL file's literal, the value reads back intact, so the loss is in the
	// write, not the read.
	typetest.NewCase("datetime64_3", "DateTime64(3)", fixt.DatetimeZ, chDatetimeMillis, "DateTime64(3)",
		"DateTime64(3)", kind.Datetime).
		WithWantVal(chDatetimeMillis.Truncate(time.Second)).
		WithWantLiteralVal(chDatetimeMillis).
		WithWantDefault("'2017-07-07 07:07:07.123'"),

	// candidate issue (#1388): IPv4 and IPv6 are kind text in inspect, but querying
	// them fails: the driver returns a net.IP, which sq can't scan.
	typetest.NewCase("ipv4", "IPv4", "0.0.0.0", "192.168.0.1", "IPv4", "IPv4", kind.Text).
		WithWant(typetest.Want{QueryErr: typetest.Is(chNetIPScanErr)}).
		WithWantDefault("'192.168.0.1'"),
	typetest.NewCase("ipv6", "IPv6", "::", "2001:db8::1", "IPv6", "IPv6", kind.Text).
		WithWant(typetest.Want{QueryErr: typetest.Is(chNetIPScanErr)}).
		WithWantDefault("'2001:db8::1'"),

	// Composite types can't be wrapped in Nullable, so these have only the
	// NOT NULL and Defaulted variants. Array values are flattened to comma-separated text:
	// #545. candidate issue (#1388): querying a Map or Tuple column fails, since the
	// driver returns a Go map or slice, which sq can't scan.
	typetest.NewCase("array_int32", "Array(Int32)", typetest.SQL("[]"), typetest.SQL("[1, 7]"), "Array(Int32)",
		"Array(Int32)", kind.Text).
		WithVariants(typetest.NotNull, typetest.Defaulted).
		WithWantZero("").
		WithWantVal("1,7").
		WithIssue("#545").
		WithWantDefault("[1, 7]"),
	typetest.NewCase("map_string_int32", "Map(String, Int32)", typetest.SQL("map()"), typetest.SQL("map('k', 7)"),
		"Map(String, Int32)", "Map(String, Int32)", kind.Text).
		WithVariants(typetest.NotNull, typetest.Defaulted).
		WithWant(typetest.Want{
			QueryErr: typetest.Is("unsupported Scan, storing driver.Value type map[string]int32 into type *string"),
			Zero:     typetest.WantNull(),
			Val:      typetest.WantNull(),
		}).
		WithWantDefault("map('k', 7)"),
	typetest.NewCase("tuple", "Tuple(Int32, String)", typetest.SQL("tuple(0, '')"), typetest.SQL("tuple(7, 'seven')"),
		"Tuple(Int32, String)", "Tuple(Int32, String)", kind.Text).
		WithVariants(typetest.NotNull, typetest.Defaulted).
		WithWant(typetest.Want{
			QueryErr: typetest.Is("unsupported Scan, storing driver.Value type []interface {} into type *string"),
			Zero:     typetest.WantNull(),
			Val:      typetest.WantNull(),
		}).
		WithWantDefault("tuple(7, 'seven')"),
}
