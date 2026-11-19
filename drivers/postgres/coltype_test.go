package postgres_test

import (
	"context"
	"encoding/hex"
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

// TestColumnTypes checks what sq reports for each Postgres column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, pgConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "Postgres 18", typetest.DDLTable{Name: "coltypes", Config: pgConfig})
}

func TestPgLiteral(t *testing.T) {
	col := func(decl string) typetest.Column { return typetest.Column{Case: &typetest.Case{Decl: decl}} }
	ts := time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	testCases := []struct {
		col  typetest.Column
		in   any
		want string
	}{
		{col("TIMETZ"), "07:07:07", "'07:07:07+00:00'"},
		{col("TIME WITH TIME ZONE"), "07:07:07", "'07:07:07+00:00'"},
		{col("TIME"), "07:07:07", "'07:07:07'"},
		{col("TIMESTAMPTZ(3)"), ts, "'2017-07-07 07:07:07.123+00:00'"},
		{col("TIMESTAMP WITHOUT TIME ZONE"), ts, "'2017-07-07 07:07:07.123'"},
		{col("INTEGER[]"), []int64(nil), "NULL"},
		{col("TEXT[]"), []string(nil), "NULL"},
		{col("TEXT[]"), []string{`a"b`, `c\d`, "it's"}, `'{"a\"b","c\\d","it''s"}'`},
		{col("BYTEA"), []byte("se"), `'\x7365'::bytea`},
	}
	for _, tc := range testCases {
		got, err := typetest.RenderLiteral(pgConfig, tc.col, tc.in)
		require.NoError(t, err, "%s %#v", tc.col.Case.Decl, tc.in)
		require.Equal(t, tc.want, got, "%s %#v", tc.col.Case.Decl, tc.in)
	}
}

// pgConfig is the typetest configuration for Postgres.
var pgConfig = typetest.Config{
	Handle:        sakila.Pg,
	RowIDDecl:     "INTEGER NOT NULL PRIMARY KEY",
	Cases:         pgTypeCases,
	CheckSettings: checkPgSettings,
	Literal:       pgLiteral,
	// Load the generated DDL in an odd time zone, so a time literal without
	// an offset fails the check. See typetest.Config.DDLCheckSession.
	DDLCheckSession: []string{"SET TIME ZONE '" + typetest.CheckTimeZone + "'"},
}

// pgLiteral renders the inputs Postgres writes differently from
// typetest.DefaultLiteral: bytes, as a bytea hex literal ('\x7365'::bytea);
// a time or time of day for a column with a time zone (TIMESTAMPTZ, TIMETZ)
// with an explicit offset, since Postgres reads one without an offset in the
// session's time zone; and arrays, in Postgres's array literal form ('{1,7}'),
// or NULL for a nil array. The bytea and array literals assume
// standard_conforming_strings, which checkPgSettings checks.
func pgLiteral(col typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case []byte:
		if v == nil {
			return "", false
		}
		return "'\\x" + hex.EncodeToString(v) + "'::bytea", true
	case time.Time:
		if typetest.HasTimeZone(col.Case.Decl) {
			return typetest.TimestampTZLiteral(v), true
		}
	case string:
		if typetest.HasTimeZone(col.Case.Decl) {
			return "'" + strings.ReplaceAll(v, "'", "''") + "+00:00'", true
		}
	case []int64:
		if v == nil {
			return "NULL", true
		}
		elems := make([]string, len(v))
		for i, e := range v {
			elems[i] = strconv.FormatInt(e, 10)
		}
		return "'{" + strings.Join(elems, ",") + "}'", true
	case []string:
		if v == nil {
			return "NULL", true
		}
		elems := make([]string, len(v))
		for i, e := range v {
			elems[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(e, `\`, `\\`), `"`, `\"`) + `"`
		}
		return "'" + strings.ReplaceAll("{"+strings.Join(elems, ",")+"}", "'", "''") + "'", true
	}
	return "", false
}

var (
	pgTimestampMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	pgNumeric5_2Max   = decimal.RequireFromString("999.99")
	pgNumeric10Max    = decimal.RequireFromString("9999999999")
)

// pgBitDefaultBefore11 returns the override for a bit string column's
// default_value on servers before 11, which write a bit string constant with
// a B prefix: B'1'::"bit" where 11 and later write '1'::"bit".
func pgBitDefaultBefore11(def string) typetest.Override {
	return typetest.Override{
		Before:     "v11.0.0",
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is(def)}},
	}
}

var pgTypeCases = []typetest.Case{
	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(math.MaxInt16), "int2", "smallint", kind.Int).
		WithWantDefault("32767"),
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, int64(math.MaxInt32), "int4", "integer", kind.Int).
		WithWantDefault("2147483647"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "int8", "bigint", kind.Int).
		WithWantDefault("'9223372036854775807'::bigint"),

	// The serial types are integers with a sequence as their default. They
	// can't be NULL, so there's no nullable variant.
	typetest.NewCase("smallserial", "SMALLSERIAL", fixt.IntZ, int64(math.MaxInt16), "int2", "smallint", kind.Int).
		WithVariants(typetest.NotNull).
		WithWant(typetest.Want{Default: typetest.Is(serialDefault("smallserial"))}),
	typetest.NewCase("serial", "SERIAL", fixt.IntZ, int64(math.MaxInt32), "int4", "integer", kind.Int).
		WithVariants(typetest.NotNull).
		WithWant(typetest.Want{Default: typetest.Is(serialDefault("serial"))}),
	typetest.NewCase("bigserial", "BIGSERIAL", fixt.IntZ, int64(math.MaxInt64), "int8", "bigint", kind.Int).
		WithVariants(typetest.NotNull).
		WithWant(typetest.Want{Default: typetest.Is(serialDefault("bigserial"))}),

	// From here on, column_type lacks the declared parameters wherever the
	// type has them (VARCHAR(16) is reported as "character varying"): #1365.

	typetest.NewCase("numeric", "NUMERIC", fixt.DecimalZ, fixt.Decimal, "numeric", "numeric", kind.Decimal).
		WithWantDefault("77.77"),
	typetest.NewCase("numeric_5_2", "NUMERIC(5,2)", fixt.DecimalZ, pgNumeric5_2Max, "numeric", "numeric",
		kind.Decimal).
		WithWantZero(decimal.New(0, -2)).
		WithIssue("#1365").
		WithWantDefault("999.99"),
	typetest.NewCase("numeric_10", "NUMERIC(10)", fixt.DecimalZ, pgNumeric10Max, "numeric", "numeric",
		kind.Decimal).
		WithIssue("#1365").
		WithWantDefault("'9999999999'::bigint"),

	// REAL is single precision.
	typetest.NewCase("real", "REAL", fixt.FloatZ, fixt.Float, "float4", "real", kind.Float).
		WithWantVal(float64(float32(fixt.Float))).
		WithWantDefault("7.7"),
	typetest.NewCase("double_precision", "DOUBLE PRECISION", fixt.FloatZ, fixt.Float, "float8", "double precision",
		kind.Float).
		WithWantDefault("7.7"),

	// candidate issue: MONEY is kind decimal in inspect, but its query result
	// kind is unknown, and its values come back as formatted text.
	typetest.NewCase("money", "MONEY", fixt.MoneyZ, fixt.Money, "money", "money", kind.Decimal).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Unknown),
			Zero:      typetest.Is[any]("$0.00"),
			Val:       typetest.Is[any]("$77.77"),
		}).
		WithWantDefault("77.77"),

	// CHARACTER pads values to its length with spaces.
	typetest.NewCase("character_255", "CHARACTER(255)", fixt.TextZ, fixt.Text, "bpchar", "character", kind.Text).
		WithWantZero(typetest.SpacePadded(fixt.TextZ, 255)).
		WithWantVal(typetest.SpacePadded(fixt.Text, 255)).
		WithIssue("#1365").
		WithWantDefault("'seven'::bpchar"),
	typetest.NewCase("varchar", "VARCHAR", fixt.TextZ, fixt.Text, "varchar", "character varying", kind.Text).
		WithWantDefault("'seven'::character varying"),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "character varying",
		kind.Text).
		WithIssue("#1365").
		WithWantDefault("'0123456789abcdef'::character varying"),
	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text", kind.Text).
		WithWantDefault("'seven'::text"),
	typetest.NewCase("bytea", "BYTEA", fixt.BytesZ, fixt.Bytes, "bytea", "bytea", kind.Bytes).
		WithWantDefault("'\\x736576656e'::bytea"),
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "bool", "boolean", kind.Bool).
		WithWantDefault("true"),

	typetest.NewCase("bit", "BIT", "0", "1", "bit", "bit", kind.Text).
		WithWantDefault("'1'::\"bit\"").
		WithVersionOverrides(pgBitDefaultBefore11("B'1'::\"bit\"")),
	typetest.NewCase("bit_8", "BIT(8)", "00000000", "10101010", "bit", "bit", kind.Text).
		WithIssue("#1365").
		WithWantDefault("'10101010'::\"bit\"").
		WithVersionOverrides(pgBitDefaultBefore11("B'10101010'::\"bit\"")),
	typetest.NewCase("bit_varying", "BIT VARYING(255)", fixt.BitStringZ, fixt.BitString, "varbit", "bit varying",
		kind.Text).
		WithIssue("#1365").
		WithWantDefault("'1001'::bit varying").
		WithVersionOverrides(pgBitDefaultBefore11("B'1001'::bit varying")),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "date", "date", kind.Date).
		WithWantDefault("'2017-07-07'::date"),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "timestamp",
		"timestamp without time zone", kind.Datetime).
		WithWantDefault("'2017-07-07 07:07:07'::timestamp without time zone"),
	typetest.NewCase("timestamp_3", "TIMESTAMP(3)", fixt.DatetimeZ, pgTimestampMillis, "timestamp",
		"timestamp without time zone", kind.Datetime).
		WithIssue("#1365").
		WithWantDefault("'2017-07-07 07:07:07.123'::timestamp without time zone"),

	// candidate issue: TIMESTAMPTZ values come back in the local time zone of
	// the machine running sq, not UTC.
	typetest.NewCase("timestamptz", "TIMESTAMPTZ", fixt.DatetimeZ, fixt.Datetime, "timestamptz",
		"timestamp with time zone", kind.Datetime).
		WithWantZero(fixt.DatetimeZ.In(time.Local)).
		WithWantVal(fixt.Datetime.In(time.Local)).
		WithWantDefault("'2017-07-07 07:07:07+00'::timestamp with time zone"),
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time without time zone", kind.Time).
		WithWantDefault("'07:07:07'::time without time zone"),
	typetest.NewCase("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time without time zone",
		kind.Time).
		WithIssue("#1365").
		WithWantDefault("'12:34:56.789'::time without time zone"),

	// candidate issue: TIMETZ is kind time in inspect, but its query result
	// kind is unknown.
	typetest.NewCase("timetz", "TIMETZ", fixt.TimeOfDayZ, fixt.TimeOfDay, "timetz", "time with time zone",
		kind.Time).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Unknown),
			Zero:      typetest.Is[any]("00:00:00+00"),
			Val:       typetest.Is[any]("07:07:07+00"),
		}).
		WithWantDefault("'07:07:07+00'::time with time zone"),

	// INTERVAL values come back in the "postgres" IntervalStyle.
	typetest.NewCase("interval", "INTERVAL", "0 seconds", "7 seconds", "interval", "interval", kind.Text).
		WithWantZero("00:00:00").
		WithWantVal("00:00:07").
		WithWantDefault("'00:00:07'::interval"),

	// The geometric and network types read back in Postgres's canonical
	// text form.

	typetest.NewCase("box", "BOX", "(0,0), (0,0)", "(0,0), (7,7)", "box", "box", kind.Text).
		WithWantZero("(0,0),(0,0)").
		WithWantVal("(7,7),(0,0)").
		WithWantDefault("'(7,7),(0,0)'::box"),
	typetest.NewCase("circle", "CIRCLE", "((0,0), 0)", "((0,0), 7)", "circle", "circle", kind.Text).
		WithWantZero("<(0,0),0>").
		WithWantVal("<(0,0),7>").
		WithWantDefault("'<(0,0),7>'::circle"),
	typetest.NewCase("line", "LINE", "[(0,0),(1,0)]", "[(0,0),(1,1)]", "line", "line", kind.Text).
		WithWantZero("{0,-1,0}").
		WithWantVal("{1,-1,0}").
		WithWantDefault("'{1,-1,0}'::line"),
	typetest.NewCase("lseg", "LSEG", "[(0,0),(0,0)]", "[(0,0),(7,0)]", "lseg", "lseg", kind.Text).
		WithWantDefault("'[(0,0),(7,0)]'::lseg"),
	typetest.NewCase("path", "PATH", "[(0,0),(0,0)]", "[(0,0),(7,0)]", "path", "path", kind.Text).
		WithWantDefault("'[(0,0),(7,0)]'::path"),
	typetest.NewCase("point", "POINT", "(0,0)", "(7,7)", "point", "point", kind.Text).
		WithWantDefault("'(7,7)'::point"),
	typetest.NewCase("polygon", "POLYGON", "((0,0))", "((0,0),(0,7),(7,0))", "polygon", "polygon", kind.Text).
		WithWantDefault("'((0,0),(0,7),(7,0))'::polygon"),

	typetest.NewCase("cidr", "CIDR", "0.0.0.0", "192.168.0.1", "cidr", "cidr", kind.Text).
		WithWantZero("0.0.0.0/32").
		WithWantVal("192.168.0.1/32").
		WithWantDefault("'192.168.0.1/32'::cidr"),
	typetest.NewCase("inet", "INET", "0.0.0.0", "192.168.0.1", "inet", "inet", kind.Text).
		WithWantDefault("'192.168.0.1'::inet"),
	typetest.NewCase("macaddr", "MACADDR", "00:00:00:00:00:00", "07:07:07:07:07:07", "macaddr", "macaddr",
		kind.Text).
		WithWantDefault("'07:07:07:07:07:07'::macaddr"),

	// candidate issue: MACADDR8, TSQUERY and PG_LSN have no kind, while their
	// siblings MACADDR and TSVECTOR are text.
	typetest.NewCase("macaddr8", "MACADDR8", "00:00:00:00:00:00:00:00", "07:07:07:07:07:07:07:07", "macaddr8",
		"macaddr8", kind.Unknown).
		WithMinVer("v10.0.0").
		WithWantDefault("'07:07:07:07:07:07:07:07'::macaddr8"),

	typetest.NewCase("uuid", "UUID", "00000000-0000-0000-0000-000000000000", "77777777-7777-7777-7777-777777777777",
		"uuid", "uuid", kind.Text).
		WithWantDefault("'77777777-7777-7777-7777-777777777777'::uuid"),
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "json", "json", kind.Text).
		WithWantDefault("'{\"val\": 7}'::json"),
	typetest.NewCase("jsonb", "JSONB", fixt.JSONZ, fixt.JSON, "jsonb", "jsonb", kind.Text).
		WithWantDefault("'{\"val\": 7}'::jsonb"),
	typetest.NewCase("xml", "XML", "", "<val>7</val>", "xml", "xml", kind.Text).
		WithWantDefault("'<val>7</val>'::xml"),
	typetest.NewCase("tsvector", "TSVECTOR", "", "alfa bravo charlie delta", "tsvector", "tsvector", kind.Text).
		WithWantVal("'alfa' 'bravo' 'charlie' 'delta'").
		WithWantDefault("'''alfa'' ''bravo'' ''charlie'' ''delta'''::tsvector"),
	typetest.NewCase("tsquery", "TSQUERY", "", "alfa | bravo", "tsquery", "tsquery", kind.Unknown).
		WithWantVal("'alfa' | 'bravo'").
		WithWantDefault("'''alfa'' | ''bravo'''::tsquery"),
	typetest.NewCase("pg_lsn", "PG_LSN", "0/0", "7/7", "pg_lsn", "pg_lsn", kind.Unknown).
		WithWantDefault("'7/7'::pg_lsn"),

	// candidate issue: arrays and ranges have no kind. Their values come back
	// as Postgres's text form. An array's column_type is just ARRAY, without
	// its element type: the information_schema data_type loss #1365
	// describes.
	typetest.NewCase("integer_array", "INTEGER[]", []int64{}, []int64{1, 7}, "_int4", "ARRAY", kind.Unknown).
		WithWantZero("{}").
		WithWantVal("{1,7}").
		WithWantDefault("'{1,7}'::integer[]"),
	typetest.NewCase("text_array", "TEXT[]", []string{}, []string{"alfa", "bravo"}, "_text", "ARRAY",
		kind.Unknown).
		WithWantZero("{}").
		WithWantVal("{alfa,bravo}").
		WithWantDefault("'{alfa,bravo}'::text[]"),
	typetest.NewCase("int4range", "INT4RANGE", "empty", "[1,7)", "int4range", "int4range", kind.Unknown).
		WithWantDefault("'[1,7)'::int4range"),
}

// serialDefault returns the default_value of a serial column of the
// given case name: a call to nextval on the sequence the column owns.
func serialDefault(caseName string) string {
	return "nextval('" + typetest.TableToken + "_col_" + caseName + "_seq'::regclass)"
}

// pgSettings are the session settings the expectations in pgTypeCases
// assume: those of the sakiladb/postgres images.
type pgSettings struct {
	timeZone      string
	dateStyle     string
	intervalStyle string
	lcMonetary    string
	stdStrings    string
}

// checkPgSettings verifies that the session's settings are those the
// expectations assume. TimeZone decides how timestamptz and timetz values
// read back, DateStyle and IntervalStyle how date and interval values are
// formatted, lc_monetary how money values are formatted, and
// standard_conforming_strings how pgLiteral's bytea and array literals are
// read.
func checkPgSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got pgSettings
	err := db.QueryRowContext(ctx,
		"SELECT current_setting('TimeZone'), current_setting('DateStyle'), "+
			"current_setting('IntervalStyle'), current_setting('lc_monetary'), "+
			"current_setting('standard_conforming_strings')").
		Scan(&got.timeZone, &got.dateStyle, &got.intervalStyle, &got.lcMonetary, &got.stdStrings)
	if err != nil {
		return errz.Err(err)
	}

	want := pgSettings{
		timeZone:      "UTC",
		dateStyle:     "ISO, MDY",
		intervalStyle: "postgres",
		lcMonetary:    "en_US.utf8",
		stdStrings:    "on",
	}
	if got != want {
		return errz.Errorf("Postgres %s: settings are %+v, expected %+v", serverVer, got, want)
	}
	return nil
}
