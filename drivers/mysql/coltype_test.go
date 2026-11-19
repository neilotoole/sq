package mysql_test

import (
	"context"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
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

// mysqlConfig is the typetest configuration for TestColumnTypes.
var mysqlConfig = typetest.Config{
	Handle:        sakila.My,
	RowIDDecl:     "INT NOT NULL PRIMARY KEY",
	Cases:         mysqlTypeCases,
	CheckSettings: checkMySQLSettings,
	Literal:       mysqlLiteral,
}

// mysqlLiteral renders the inputs MySQL writes differently from
// typetest.DefaultLiteral: bytes, as a hex literal (X'7365').
func mysqlLiteral(_ typetest.Column, v any) (string, bool) {
	if b, ok := v.([]byte); ok && b != nil {
		return "X'" + hex.EncodeToString(b) + "'", true
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
	Handle:        sakila.My,
	RowIDDecl:     "INT NOT NULL PRIMARY KEY",
	Cases:         []typetest.Case{mysqlTimestamp6Case},
	CheckSettings: checkMySQLSettings,
	Literal:       mysqlLiteral,
}

// mysqlTimestamp6Case is the TIMESTAMP(6) case for TestColumnTypes_TIMESTAMP6.
// On the 5.x images its NOT NULL column, the first TIMESTAMP column in its
// table, gets the implicit default MySQL gives that column.
var mysqlTimestamp6Case = typetest.NewCase("timestamp_6", "TIMESTAMP(6)", fixt.DatetimeZ, datetimeMicros,
	"timestamp", "timestamp(6)", kind.Datetime).
	WithVersionOverrides(typetest.Override{
		Before:     "v8.0.0",
		PerVariant: typetest.PerVariant{NotNull: typetest.Want{Default: typetest.Is("CURRENT_TIMESTAMP(6)")}},
	})

var mysqlTypeCases = []typetest.Case{
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(127), "tinyint", "tinyint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("tinyint(4)")),

	// BOOLEAN is stored as TINYINT(1). It's reported as kind int, and its
	// values come back as 1 and 0.
	typetest.NewCase("tinyint_1", "TINYINT(1)", fixt.IntZ, int64(1), "tinyint", "tinyint(1)", kind.Int).
		WithIssue("#1365"),

	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(32767), "smallint", "smallint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("smallint(6)")),
	typetest.NewCase("mediumint", "MEDIUMINT", fixt.IntZ, int64(8388607), "mediumint", "mediumint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("mediumint(9)")),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(11)")),
	typetest.NewCase("int_11", "INT(11)", fixt.IntZ, fixt.Int, "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(11)")),
	typetest.NewCase("int_24", "INT(24)", fixt.IntZ, fixt.Int, "int", "int", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(24)")),
	typetest.NewCase("int_unsigned", "INT UNSIGNED", fixt.IntZ, int64(math.MaxUint32), "int", "int unsigned", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("int(10) unsigned")),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "bigint", "bigint", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("bigint(20)")),

	// candidate issue: the NOT NULL column wraps values above math.MaxInt64 to
	// negative int64 (the maximum reads back as -1), and querying the nullable
	// column fails: the driver scans it as sql.Null[uint64], which sq doesn't
	// handle.
	typetest.NewCase("bigint_unsigned", "BIGINT UNSIGNED", fixt.IntZ, uint64(math.MaxUint64),
		"bigint", "bigint unsigned", kind.Int).
		WithVersionOverrides(displayWidthBefore8019("bigint(20) unsigned")).
		WithPerVariant(typetest.PerVariant{
			NotNull:  typetest.Want{Val: typetest.Is[any](int64(-1))},
			Nullable: typetest.Want{QueryErr: typetest.Is("unknown type db(*sql.Null[uint64])")},
		}),

	// candidate issue: BIT is reported as kind int, but its values come back as
	// a string of the raw bytes.
	typetest.NewCase("bit_1", "BIT(1)", fixt.IntZ, int64(1), "bit", "bit(1)", kind.Int).
		WithWantZero("\x00").
		WithWantVal("\x01"),
	typetest.NewCase("bit_64", "BIT(64)", fixt.IntZ, fixt.Int, "bit", "bit(64)", kind.Int).
		WithWantZero("\x00\x00\x00\x00\x00\x00\x00\x00").
		WithWantVal("\x00\x00\x00\x00\x00\x00\x00\x07"),

	// DECIMAL is DECIMAL(10,0): MySQL rounds 77.77 to 78.
	typetest.NewCase("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "decimal", "decimal(10,0)", kind.Decimal).
		WithWantVal(decimal.New(78, 0)),
	typetest.NewCase("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, decimal5_2Max, "decimal", "decimal(5,2)",
		kind.Decimal).
		WithWantZero(decimal.New(0, -2)),
	typetest.NewCase("decimal_10", "DECIMAL(10)", fixt.DecimalZ, decimal10Max, "decimal", "decimal(10,0)",
		kind.Decimal),

	// FLOAT is single precision. candidate issue: the NOT NULL column returns
	// the float32 value widened (7.699999809265137), while the nullable column
	// returns 7.7.
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "float", "float", kind.Float).
		WithPerVariant(typetest.PerVariant{
			NotNull: typetest.Want{Val: typetest.Is[any](float64(float32(fixt.Float)))},
		}),

	typetest.NewCase("double", "DOUBLE", fixt.FloatZ, fixt.Float, "double", "double", kind.Float),
	typetest.NewCase("char_255", "CHAR(255)", fixt.TextZ, fixt.Text, "char", "char(255)", kind.Text),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "varchar(16)",
		kind.Text),
	typetest.NewCase("varchar_255", "VARCHAR(255)", fixt.TextZ, fixt.Text, "varchar", "varchar(255)", kind.Text),
	typetest.NewCase("tinytext", "TINYTEXT", fixt.TextZ, fixt.Text, "tinytext", "tinytext", kind.Text),
	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text", kind.Text),
	typetest.NewCase("mediumtext", "MEDIUMTEXT", fixt.TextZ, fixt.Text, "mediumtext", "mediumtext", kind.Text),
	typetest.NewCase("longtext", "LONGTEXT", fixt.TextZ, fixt.Text, "longtext", "longtext", kind.Text),

	// BINARY pads values to its length with zero bytes.
	typetest.NewCase("binary_64", "BINARY(64)", fixt.BytesZ, fixt.Bytes, "binary", "binary(64)", kind.Bytes).
		WithWantZero(typetest.ZeroPadded(fixt.BytesZ, 64)).
		WithWantVal(typetest.ZeroPadded(fixt.Bytes, 64)),

	typetest.NewCase("varbinary_64", "VARBINARY(64)", fixt.BytesZ, fixt.Bytes, "varbinary", "varbinary(64)",
		kind.Bytes),
	typetest.NewCase("tinyblob", "TINYBLOB", fixt.BytesZ, fixt.Bytes, "tinyblob", "tinyblob", kind.Bytes),
	typetest.NewCase("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "blob", "blob", kind.Bytes),
	typetest.NewCase("mediumblob", "MEDIUMBLOB", fixt.BytesZ, fixt.Bytes, "mediumblob", "mediumblob", kind.Bytes),
	typetest.NewCase("longblob", "LONGBLOB", fixt.BytesZ, fixt.Bytes, "longblob", "longblob", kind.Bytes),
	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "date", "date", kind.Date),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "datetime", "datetime", kind.Datetime),
	typetest.NewCase("datetime_6", "DATETIME(6)", fixt.DatetimeZ, datetimeMicros, "datetime", "datetime(6)",
		kind.Datetime),

	// With explicit_defaults_for_timestamp off (the 5.x images), the first
	// TIMESTAMP column in a table, if it's NOT NULL and declares no default,
	// gets an implicit one. col_timestamp is that column: keep it the first
	// TIMESTAMP case. See TestColumnTypes_TIMESTAMP6.
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "timestamp", "timestamp",
		kind.Datetime).
		WithVersionOverrides(typetest.Override{
			Before:     "v8.0.0",
			PerVariant: typetest.PerVariant{NotNull: typetest.Want{Default: typetest.Is("CURRENT_TIMESTAMP")}},
		}),

	// TIMESTAMP(6) isn't here: it needs a table of its own on the 5.x images.
	// See TestColumnTypes_TIMESTAMP6.

	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time", kind.Time),
	typetest.NewCase("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time(3)", kind.Time).
		WithWantZero("00:00:00.000"),

	// MySQL reads a YEAR of 1 to 69 as 2001 to 2069.
	typetest.NewCase("year", "YEAR", fixt.IntZ, fixt.Int, "year", "year", kind.Int).
		WithWantVal(int64(2007)).
		WithVersionOverrides(displayWidthBefore8019("year(4)")),

	typetest.NewCase("enum", "ENUM('alfa','bravo','charlie')", fixt.EnumAlfa, fixt.EnumBravo,
		"enum", "enum('alfa','bravo','charlie')", kind.Text),
	typetest.NewCase("set", "SET('alfa','bravo','charlie')", fixt.EnumAlfa, "alfa,bravo",
		"set", "set('alfa','bravo','charlie')", kind.Text),
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "json", "json", kind.Text).
		WithMinVer("v5.7.8"),

	// candidate issue: spatial types have no kind, and their values come back
	// as a string of MySQL's internal format (a 4-byte SRID, then WKB) rather
	// than as WKB or text.
	typetest.NewCase("geometry", "GEOMETRY", point11, point22, "geometry", "geometry", kind.Unknown).
		WithWantZero(srid0 + point11WKB).
		WithWantVal(srid0 + point22WKB),
	typetest.NewCase("point", "POINT", point11, point22, "point", "point", kind.Unknown).
		WithWantZero(srid0 + point11WKB).
		WithWantVal(srid0 + point22WKB),
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

// checkMySQLSettings verifies that the server's settings are those the
// expectations assume. explicit_defaults_for_timestamp decides the implicit
// defaults of TIMESTAMP columns, and sql_mode decides how zero and edge
// inputs are stored.
func checkMySQLSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got mysqlSettings
	err := db.QueryRowContext(ctx, "SELECT @@explicit_defaults_for_timestamp, @@sql_mode").
		Scan(&got.explicitDefaultsForTimestamp, &got.sqlMode)
	if err != nil {
		return errz.Err(err)
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
