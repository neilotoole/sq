package mysql_test

import (
	"context"
	"database/sql/driver"
	"encoding/hex"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/neilotoole/sq/testh/typetest"
)

// TestColumnTypes checks what sq reports for each MySQL column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, mysqlConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// tables of TestColumnTypes and TestColumnTypes_TIMESTAMP6, which the file
// creates as two tables. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "MySQL 9",
		typetest.DDLTable{Name: "coltypes", Config: mysqlConfig},
		typetest.DDLTable{Name: "coltypes_timestamp6", Config: mysqlTimestamp6Config})
}

func TestMySQLDefaultExpr(t *testing.T) {
	col := func(decl string, val any) typetest.Column {
		return typetest.Column{Case: &typetest.Case{Decl: decl, Val: val}}
	}
	require.Equal(t, "7", mysqlDefaultExpr(col("INT", int64(7)), "7"))
	require.Equal(t, "'seven'", mysqlDefaultExpr(col("VARCHAR(16)", "seven"), "'seven'"))
	// MySQL 8.0.13+ takes a default for these types only as an expression.
	require.Equal(t, "('seven')", mysqlDefaultExpr(col("TEXT", "seven"), "'seven'"))
	require.Equal(t, "('seven')", mysqlDefaultExpr(col("LONGTEXT", "seven"), "'seven'"))
	require.Equal(t, "(X'7365')", mysqlDefaultExpr(col("BLOB", []byte("se")), "X'7365'"))
	require.Equal(t, `('{"val": 7}')`, mysqlDefaultExpr(col("JSON", `{"val": 7}`), `'{"val": 7}'`))
	// A SQL input is an expression, whatever the type.
	require.Equal(t, "(ST_GeomFromText('POINT(2 2)'))",
		mysqlDefaultExpr(col("POINT", point22), "ST_GeomFromText('POINT(2 2)')"))
}

// TestCheckMySQLSettings_TimeZone checks that checkMySQLSettings rejects a
// session whose time zone isn't UTC: MySQL reads a TIMESTAMP default literal
// in the session's time zone when it creates the table.
func TestCheckMySQLSettings_TimeZone(t *testing.T) {
	tu.SkipShort(t, true)
	th := testh.New(t)
	if !th.SourceConfigured(sakila.My) {
		t.Skipf("source %s not configured", sakila.My)
	}
	src := th.Source(sakila.My)
	serverVer, err := th.Open(src).DBSemver(th.Context)
	require.NoError(t, err)

	conn, err := th.OpenDB(src).Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Discard the connection, so its session time zone doesn't leak
		// into the pool.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	})

	// Europe/London is at UTC+0 in winter, but reads the July 2017 literals
	// at +01:00: the check is of the zone, not of today's offset.
	for _, tz := range []string{"+05:45", "Europe/London"} {
		_, err = conn.ExecContext(th.Context, "SET time_zone = '"+tz+"'")
		require.NoError(t, err)
		err = checkMySQLSettings(th.Context, conn, serverVer)
		require.Error(t, err, tz)
		require.Contains(t, err.Error(), "time zone", tz)
	}

	_, err = conn.ExecContext(th.Context, "SET time_zone = 'SYSTEM'")
	require.NoError(t, err)
	require.NoError(t, checkMySQLSettings(th.Context, conn, serverVer), "the images' system time zone is UTC")

	_, err = conn.ExecContext(th.Context, "SET time_zone = '+00:00'")
	require.NoError(t, err)
	require.NoError(t, checkMySQLSettings(th.Context, conn, serverVer))
}

func TestMySQLLiteral(t *testing.T) {
	// MySQL reads a backslash in a string literal as an escape.
	got, ok := mysqlLiteral(typetest.Column{Case: &typetest.Case{Decl: "TEXT"}}, `a\nb'c`)
	require.True(t, ok)
	require.Equal(t, `'a\\nb''c'`, got)
}

// mysqlConfig is the typetest configuration for TestColumnTypes.
var mysqlConfig = typetest.Config{
	Handle:          sakila.My,
	RowIDDecl:       "INT NOT NULL PRIMARY KEY",
	Cases:           mysqlTypeCases,
	CheckSettings:   checkMySQLSettings,
	Literal:         mysqlLiteral,
	DefaultExpr:     mysqlDefaultExpr,
	DDLPrelude:      []string{mysqlDDLPrelude},
	DDLCheckSession: []string{mysqlCheckSession},
}

// mysqlDDLPrelude starts the generated DDL file: MySQL reads a TIMESTAMP
// literal in the session's time zone, and the literals are UTC.
const mysqlDDLPrelude = "SET time_zone = '+00:00'"

// binaryDefaultBefore8 returns the override for a BINARY or VARBINARY
// column's default_value on 5.x servers, which report the default's raw
// bytes, a BINARY column's NUL padding included, where 8.4 reports it in hex
// (0x736576656E). The exact version of the change is unknown: the images are
// 5.7.44 and 8.4.10.
//
// candidate issue: on 5.x, default_value holds raw bytes, NULs and all.
func binaryDefaultBefore8(def string) typetest.Override {
	return typetest.Override{
		Before:     "v8.0.0",
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is(def)}},
	}
}

// mysqlNoDefaultBefore8013 drops the Defaulted column on servers before
// 8.0.13, which reject any default on a BLOB, TEXT, JSON or spatial column
// (see mysqlNeedsExprDefault).
var mysqlNoDefaultBefore8013 = typetest.Override{
	Before:   "v8.0.13",
	Variants: []typetest.Variant{typetest.NotNull, typetest.Nullable},
}

// mysqlDefaultExpr writes lit as the default expression of a Defaulted
// column. MySQL 8.0.13+ accepts a default on a BLOB, TEXT, JSON or spatial
// column only as an expression, in parentheses, and needs parentheses around
// any expression default, such as a SQL input. Other literals are used as is.
func mysqlDefaultExpr(col typetest.Column, lit string) string {
	if _, ok := col.Case.Val.(typetest.SQL); ok || mysqlNeedsExprDefault(col.Case.Decl) {
		return "(" + lit + ")"
	}
	return lit
}

// mysqlNeedsExprDefault reports whether a column declared decl can take a
// default only as an expression: a BLOB, TEXT, JSON or spatial column. Before
// 8.0.13, MySQL rejects any default on those types.
func mysqlNeedsExprDefault(decl string) bool {
	switch base, _, _ := strings.Cut(strings.ToUpper(decl), "("); base {
	case "TINYBLOB", "BLOB", "MEDIUMBLOB", "LONGBLOB",
		"TINYTEXT", "TEXT", "MEDIUMTEXT", "LONGTEXT",
		"JSON",
		"GEOMETRY", "POINT", "LINESTRING", "POLYGON",
		"MULTIPOINT", "MULTILINESTRING", "MULTIPOLYGON", "GEOMETRYCOLLECTION":
		return true
	default:
		return false
	}
}

// mysqlCheckSession sets the ddl subtest's session to an odd time zone (see
// typetest.Config.DDLCheckSession), as an offset, which works on any server,
// with or without its time zone tables loaded.
const mysqlCheckSession = "SET time_zone = '" + typetest.CheckTimeZoneOffset + "'"

// mysqlLiteral renders the inputs MySQL writes differently from
// typetest.DefaultLiteral: bytes, as a hex literal (X'7365'), and strings with
// backslashes escaped, since MySQL reads a backslash in a string literal as an
// escape.
func mysqlLiteral(_ typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case []byte:
		if v != nil {
			return "X'" + hex.EncodeToString(v) + "'", true
		}
	case string:
		return "'" + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), "'", "''") + "'", true
	}
	return "", false
}

// TestColumnTypes_TIMESTAMP6 is TestColumnTypes for a single case,
// TIMESTAMP(6), which runs in a table of its own. It's the same harness and
// the same checks as TestColumnTypes; only the table differs.
//
// The reason is MySQL's handling of TIMESTAMP columns on the 5.x images,
// where explicit_defaults_for_timestamp is off (checkMySQLSettings pins
// this). With that setting off, MySQL gives TIMESTAMP columns implicit
// defaults, by position in the table:
//
//   - the first TIMESTAMP column in the table, unless it's declared NULL or
//     declares a DEFAULT or ON UPDATE, gets
//     DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
//   - every later TIMESTAMP column, unless it's declared NULL or declares a
//     DEFAULT, gets DEFAULT '0000-00-00 00:00:00'.
//
// Only the first TIMESTAMP column can get CURRENT_TIMESTAMP: if it's declared
// NULL, no column does. TestColumnTypes's first TIMESTAMP column is
// col_timestamp, declared NOT NULL, so a TIMESTAMP(6) NOT NULL column in that
// table would be a later one, and get the zero-date default. The 5.7 image's strict sql_mode
// (NO_ZERO_DATE) rejects that, so CREATE TABLE fails for the whole table:
// "Error 1067 (42000): Invalid default value for 'col_timestamp_6'". In a
// table of its own, col_timestamp_6 is the first TIMESTAMP column, gets the
// CURRENT_TIMESTAMP default instead, and the table can be created on
// every version. The nullable column, declared NULL explicitly, gets no
// implicit default on any version.
//
// MySQL 8.0 turned explicit_defaults_for_timestamp on by default, so on 8 and
// 9 neither column gets an implicit default, and TIMESTAMP(6) could share the
// main table. Keeping it here on every version keeps it one case, with one set
// of expectations.
func TestColumnTypes_TIMESTAMP6(t *testing.T) {
	t.Parallel()

	typetest.Run(t, mysqlTimestamp6Config)
}

// mysqlTimestamp6Config is the typetest configuration for
// TestColumnTypes_TIMESTAMP6.
var mysqlTimestamp6Config = typetest.Config{
	Handle:          sakila.My,
	RowIDDecl:       "INT NOT NULL PRIMARY KEY",
	Cases:           []typetest.Case{mysqlTimestamp6Case},
	CheckSettings:   checkMySQLSettings,
	Literal:         mysqlLiteral,
	DefaultExpr:     mysqlDefaultExpr,
	DDLPrelude:      []string{mysqlDDLPrelude},
	DDLCheckSession: []string{mysqlCheckSession},
}

// mysqlTimestamp6Case is the TIMESTAMP(6) case for TestColumnTypes_TIMESTAMP6.
// On the 5.x images its NOT NULL column, the first TIMESTAMP column in its
// table, gets the implicit default MySQL gives that column.
var mysqlTimestamp6Case = typetest.NewCase("timestamp_6", "TIMESTAMP(6)", fixt.DatetimeZ, datetimeMicros,
	"timestamp", "timestamp(6)", kind.Datetime).
	WithVersionOverrides(typetest.Override{
		Before:     "v8.0.0",
		PerVariant: typetest.PerVariant{NotNull: typetest.Want{Default: typetest.Is("CURRENT_TIMESTAMP(6)")}},
	}).
	WithWantDefault("2017-07-07 07:07:07.123456")

var mysqlTypeCases = []typetest.Case{
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(127), "tinyint", "tinyint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("tinyint(4)")).
		WithWantDefault("127"),

	// BOOLEAN is stored as TINYINT(1). It's reported as kind int, and its
	// values come back as 1 and 0.
	typetest.NewCase("tinyint_1", "TINYINT(1)", fixt.IntZ, int64(1), "tinyint", "tinyint(1)", kind.Int).
		WithIssue("#1365").
		WithWantDefault("1"),

	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(32767), "smallint", "smallint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("smallint(6)")).
		WithWantDefault("32767"),
	typetest.NewCase("mediumint", "MEDIUMINT", fixt.IntZ, int64(8388607), "mediumint", "mediumint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("mediumint(9)")).
		WithWantDefault("8388607"),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(11)")).
		WithWantDefault("2147483647"),
	typetest.NewCase("int_11", "INT(11)", fixt.IntZ, fixt.Int, "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(11)")).
		WithWantDefault("7"),
	typetest.NewCase("int_24", "INT(24)", fixt.IntZ, fixt.Int, "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(24)")).
		WithWantDefault("7"),
	typetest.NewCase("int_unsigned", "INT UNSIGNED", fixt.IntZ, int64(math.MaxUint32), "int", "int unsigned", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(10) unsigned")).
		WithWantDefault("4294967295"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "bigint", "bigint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("bigint(20)")).
		WithWantDefault("9223372036854775807"),

	// candidate issue: the NOT NULL column wraps values above math.MaxInt64 to
	// negative int64 (the maximum reads back as -1), and querying the nullable
	// column fails: the driver scans it as sql.Null[uint64], which sq doesn't
	// handle.
	typetest.NewCase("bigint_unsigned", "BIGINT UNSIGNED", fixt.IntZ, uint64(math.MaxUint64),
		"bigint", "bigint unsigned", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("bigint(20) unsigned")).
		WithPerVariant(typetest.PerVariant{
			NotNull:   typetest.Want{Val: typetest.Is[any](int64(-1))},
			Nullable:  typetest.Want{QueryErr: typetest.Is("unknown type db(*sql.Null[uint64])")},
			Defaulted: typetest.Want{Val: typetest.Is[any](int64(-1))},
		}).
		WithWantDefault("18446744073709551615"),

	// candidate issue: BIT is reported as kind int, but its values come back as
	// a string of the raw bytes.
	typetest.NewCase("bit_1", "BIT(1)", fixt.IntZ, int64(1), "bit", "bit(1)", kind.Int).
		WithWantZero("\x00").
		WithWantVal("\x01").
		WithWantDefault("b'1'"),
	typetest.NewCase("bit_64", "BIT(64)", fixt.IntZ, fixt.Int, "bit", "bit(64)", kind.Int).
		WithWantZero("\x00\x00\x00\x00\x00\x00\x00\x00").
		WithWantVal("\x00\x00\x00\x00\x00\x00\x00\x07").
		WithWantDefault("b'111'"),

	// DECIMAL is DECIMAL(10,0): MySQL rounds 77.77 to 78.
	typetest.NewCase("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "decimal", "decimal(10,0)", kind.Decimal).
		WithWantVal(decimal.New(78, 0)).
		WithWantDefault("78"),
	typetest.NewCase("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, decimal5_2Max, "decimal", "decimal(5,2)",
		kind.Decimal).
		WithWantZero(decimal.New(0, -2)).
		WithWantDefault("999.99"),
	typetest.NewCase("decimal_10", "DECIMAL(10)", fixt.DecimalZ, decimal10Max, "decimal", "decimal(10,0)",
		kind.Decimal).
		WithWantDefault("9999999999"),

	// FLOAT is single precision. candidate issue: the NOT NULL column returns
	// the float32 value widened (7.699999809265137), while the nullable column
	// returns 7.7.
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "float", "float", kind.Float).
		WithPerVariant(typetest.PerVariant{
			NotNull:   typetest.Want{Val: typetest.Is[any](float64(float32(fixt.Float)))},
			Defaulted: typetest.Want{Val: typetest.Is[any](float64(float32(fixt.Float)))},
		}).
		WithWantDefault("7.7"),

	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "double", "double", kind.Float).
		WithWantDefault("7.7"),
	typetest.NewCase("char_255", "CHAR(255)", fixt.TextZ, fixt.Text, "char", "char(255)", kind.Text).
		WithWantDefault("seven"),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "varchar(16)",
		kind.Text).
		WithWantDefault("0123456789abcdef"),
	typetest.NewCase("varchar_255", "VARCHAR(255)", fixt.TextZ, fixt.Text, "varchar", "varchar(255)", kind.Text).
		WithWantDefault("seven"),

	// The TEXT, BLOB, JSON and spatial types take a default only as an
	// expression (see mysqlDefaultExpr), from 8.0.13.
	//
	// candidate issue: an expression default's default_value is
	// information_schema's raw form of it, with a charset introducer and
	// escaped quotes: _utf8mb4\'seven\' for the declared ('seven').
	typetest.NewCase("tinytext", "TINYTEXT", fixt.TextZ, fixt.Text, "tinytext", "tinytext", kind.Text).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("_utf8mb4\\'seven\\'"),
	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text", kind.Text).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("_utf8mb4\\'seven\\'"),
	typetest.NewCase("mediumtext", "MEDIUMTEXT", fixt.TextZ, fixt.Text, "mediumtext", "mediumtext", kind.Text).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("_utf8mb4\\'seven\\'"),
	typetest.NewCase("longtext", "LONGTEXT", fixt.TextZ, fixt.Text, "longtext", "longtext", kind.Text).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("_utf8mb4\\'seven\\'"),

	// BINARY pads values to its length with zero bytes.
	typetest.NewCase("binary_64", "BINARY(64)", fixt.BytesZ, fixt.Bytes, "binary", "binary(64)", kind.Bytes).
		WithWantZero(typetest.ZeroPadded(fixt.BytesZ, 64)).
		WithWantVal(typetest.ZeroPadded(fixt.Bytes, 64)).
		WithWantDefault("0x736576656E").
		WithVersionOverrides(binaryDefaultBefore8(string(typetest.ZeroPadded(fixt.Bytes, 64)))),

	typetest.NewCase("varbinary_64", "VARBINARY(64)", fixt.BytesZ, fixt.Bytes, "varbinary", "varbinary(64)",
		kind.Bytes).
		WithWantDefault("0x736576656E").
		WithVersionOverrides(binaryDefaultBefore8(string(fixt.Bytes))),
	typetest.NewCase("tinyblob", "TINYBLOB", fixt.BytesZ, fixt.Bytes, "tinyblob", "tinyblob", kind.Bytes).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("0x736576656e"),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "blob", "blob", kind.Bytes).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("0x736576656e"),
	typetest.NewCase("mediumblob", "MEDIUMBLOB", fixt.BytesZ, fixt.Bytes, "mediumblob", "mediumblob", kind.Bytes).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("0x736576656e"),
	typetest.NewCase("longblob", "LONGBLOB", fixt.BytesZ, fixt.Bytes, "longblob", "longblob", kind.Bytes).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("0x736576656e"),
	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "date", "date", kind.Date).
		WithWantDefault("2017-07-07"),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "datetime", "datetime", kind.Datetime).
		WithWantDefault("2017-07-07 07:07:07"),
	typetest.NewCase("datetime_6", "DATETIME(6)", fixt.DatetimeZ, datetimeMicros, "datetime", "datetime(6)",
		kind.Datetime).
		WithWantDefault("2017-07-07 07:07:07.123456"),

	// With explicit_defaults_for_timestamp off (the 5.x images), the first
	// TIMESTAMP column in a table, if it's NOT NULL and declares no default,
	// gets an implicit one. col_timestamp is that column: keep it the first
	// TIMESTAMP case. See TestColumnTypes_TIMESTAMP6.
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "timestamp", "timestamp",
		kind.Datetime).
		WithVersionOverrides(typetest.Override{
			Before:     "v8.0.0",
			PerVariant: typetest.PerVariant{NotNull: typetest.Want{Default: typetest.Is("CURRENT_TIMESTAMP")}},
		}).
		WithWantDefault("2017-07-07 07:07:07"),

	// TIMESTAMP(6) isn't here: it needs a table of its own on the 5.x images.
	// See TestColumnTypes_TIMESTAMP6.

	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time", kind.Time).
		WithWantDefault("07:07:07"),
	typetest.NewCase("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time(3)", kind.Time).
		WithWantZero("00:00:00.000").
		WithWantDefault("12:34:56.789"),

	// MySQL reads a YEAR of 1 to 69 as 2001 to 2069.
	typetest.NewCase("year", "YEAR", fixt.IntZ, fixt.Int, "year", "year", kind.Int).
		WithWantVal(int64(2007)).
		WithVersionOverrides(displayWidthBefore8019("year(4)")).
		WithWantDefault("2007"),

	typetest.NewCase("enum", "ENUM('alfa','bravo','charlie')", fixt.EnumAlfa, fixt.EnumBravo,
		"enum", "enum('alfa','bravo','charlie')", kind.Text).
		WithWantDefault("bravo"),
	typetest.NewCase("set", "SET('alfa','bravo','charlie')", fixt.EnumAlfa, "alfa,bravo",
		"set", "set('alfa','bravo','charlie')", kind.Text).
		WithWantDefault("alfa,bravo"),
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "json", "json", kind.Text).
		WithMinVer("v5.7.8").
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("_utf8mb4\\'{\"val\": 7}\\'"),

	// candidate issue: spatial types have no kind, and their values come back
	// as a string of MySQL's internal format (a 4-byte SRID, then WKB) rather
	// than as WKB or text.
	typetest.NewCase("geometry", "GEOMETRY", point11, point22, "geometry", "geometry", kind.Unknown).
		WithWantZero(srid0 + point11WKB).
		WithWantVal(srid0 + point22WKB).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("st_geomfromtext(_utf8mb4\\'POINT(2 2)\\')"),
	typetest.NewCase("point", "POINT", point11, point22, "point", "point", kind.Unknown).
		WithWantZero(srid0 + point11WKB).
		WithWantVal(srid0 + point22WKB).
		WithVersionOverrides(mysqlNoDefaultBefore8013).
		WithWantDefault("st_geomfromtext(_utf8mb4\\'POINT(2 2)\\')"),
}

// mysqlSettings are the server settings the expectations in mysqlTypeCases
// assume: those of the sakiladb/mysql images.
type mysqlSettings struct {
	explicitDefaultsForTimestamp string
	sqlMode                      string
}

// wantMySQLSettings returns the settings expected on a server at serverVer.
func wantMySQLSettings(serverVer string) mysqlSettings {
	if semver.Compare(serverVer, "v5.7.0") < 0 {
		// The 5.6 image: sql_mode isn't strict.
		return mysqlSettings{
			explicitDefaultsForTimestamp: "0",
			sqlMode:                      "NO_ENGINE_SUBSTITUTION",
		}
	}

	if semver.Compare(serverVer, "v8.0.0") < 0 {
		// The 5.7 image: explicit_defaults_for_timestamp is off, and
		// sql_mode still has NO_AUTO_CREATE_USER (removed in 8.0).
		return mysqlSettings{
			explicitDefaultsForTimestamp: "0",
			sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE," +
				"ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION",
		}
	}

	return mysqlSettings{
		explicitDefaultsForTimestamp: "1",
		sqlMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE," +
			"ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION",
	}
}

// mysqlUTCZone reports whether a session time zone of timeZone is UTC:
// +00:00 or UTC, or SYSTEM where the server's system time zone, systemTimeZone,
// is UTC, as on the sakiladb images. Another zone that happens to be at UTC+0
// is rejected, since it may not be on every date.
func mysqlUTCZone(timeZone, systemTimeZone string) bool {
	switch timeZone {
	case "+00:00", "UTC":
		return true
	case "SYSTEM":
		return systemTimeZone == "UTC"
	default:
		return false
	}
}

// checkMySQLSettings verifies that the server's settings are those the
// expectations assume. explicit_defaults_for_timestamp decides the implicit
// defaults of TIMESTAMP columns, sql_mode decides how zero and edge inputs
// are stored, and the session time zone how a TIMESTAMP default literal is
// read.
func checkMySQLSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got mysqlSettings
	var timeZone, systemTimeZone string
	err := db.QueryRowContext(ctx,
		"SELECT @@explicit_defaults_for_timestamp, @@sql_mode, @@session.time_zone, @@system_time_zone").
		Scan(&got.explicitDefaultsForTimestamp, &got.sqlMode, &timeZone, &systemTimeZone)
	if err != nil {
		return errz.Err(err)
	}

	// MySQL reads a TIMESTAMP literal, a Defaulted column's default among
	// them, in the session's time zone, and the literals are UTC. Check the
	// zone itself, not today's offset: a zone like Europe/London is at UTC+0
	// in winter, but reads the July literals at +01:00.
	if !mysqlUTCZone(timeZone, systemTimeZone) {
		return errz.Errorf("MySQL %s: the session time zone is {%s} (system time zone {%s}), expected UTC",
			serverVer, timeZone, systemTimeZone)
	}

	want := wantMySQLSettings(serverVer)
	if got.explicitDefaultsForTimestamp != want.explicitDefaultsForTimestamp {
		return errz.Errorf("MySQL %s: explicit_defaults_for_timestamp is {%s}, expected {%s}",
			serverVer, got.explicitDefaultsForTimestamp, want.explicitDefaultsForTimestamp)
	}
	if got.sqlMode != want.sqlMode {
		return errz.Errorf("MySQL %s: sql_mode is {%s}, expected {%s}", serverVer, got.sqlMode, want.sqlMode)
	}
	return nil
}

var (
	datetimeMicros = time.Date(2017, 7, 7, 7, 7, 7, 123456000, time.UTC)
	decimal5_2Max  = decimal.RequireFromString("999.99")
	decimal10Max   = decimal.RequireFromString("9999999999")
)

// point11 and point22 are the SQL inputs for the spatial columns: POINT(1 1)
// and POINT(2 2).
const (
	point11 = typetest.SQL("ST_GeomFromText('POINT(1 1)')")
	point22 = typetest.SQL("ST_GeomFromText('POINT(2 2)')")
)

// A spatial column reads back today as a string of MySQL's internal format:
// a 4-byte SRID (srid0), then the little-endian WKB of the value
// (point11WKB, point22WKB).
const (
	srid0      = "\x00\x00\x00\x00"
	point11WKB = "\x01\x01\x00\x00\x00" + // little-endian, type 1 (point)
		"\x00\x00\x00\x00\x00\x00\xf0?" + // x = 1.0
		"\x00\x00\x00\x00\x00\x00\xf0?" // y = 1.0
	point22WKB = "\x01\x01\x00\x00\x00" + // little-endian, type 1 (point)
		"\x00\x00\x00\x00\x00\x00\x00@" + // x = 2.0
		"\x00\x00\x00\x00\x00\x00\x00@" // y = 2.0
)

// displayWidthBefore8019 returns the override for an integer or YEAR column
// type on servers before 8.0.19, which still report the display width
// (int(11), year(4)). 8.0.19 stopped reporting it, except for TINYINT(1) and
// ZEROFILL columns (no case here covers ZEROFILL).
func displayWidthBefore8019(colType string) typetest.Override {
	return typetest.Override{
		Before: "v8.0.19",
		Want:   typetest.Want{ColumnType: typetest.Is(colType)},
	}
}
