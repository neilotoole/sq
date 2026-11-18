package mysql_test

import (
	"context"
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

	typetest.Run(t, typetest.Config{
		Handle:        sakila.My,
		RowIDDecl:     "INT NOT NULL PRIMARY KEY",
		Cases:         mysqlTypeCases,
		CheckSettings: checkMySQLSettings,
	})
}

var mysqlTypeCases = []typetest.Case{
	withVersionOverrides(
		mc("tinyint", "TINYINT", fixt.IntZ, int64(127), "tinyint", "tinyint", kind.Int),
		displayWidthBefore8019("tinyint(4)"),
	),
	func() typetest.Case {
		// BOOLEAN is stored as TINYINT(1). It's reported as kind int, and its
		// values come back as 1 and 0.
		c := mc("tinyint_1", "TINYINT(1)", fixt.IntZ, int64(1), "tinyint", "tinyint(1)", kind.Int)
		c.Issue = "#1365"
		return c
	}(),
	withVersionOverrides(
		mc("smallint", "SMALLINT", fixt.IntZ, int64(32767), "smallint", "smallint", kind.Int),
		displayWidthBefore8019("smallint(6)"),
	),
	withVersionOverrides(
		mc("mediumint", "MEDIUMINT", fixt.IntZ, int64(8388607), "mediumint", "mediumint", kind.Int),
		displayWidthBefore8019("mediumint(9)"),
	),
	withVersionOverrides(
		mc("int", "INT", fixt.IntZ, int64(math.MaxInt32), "int", "int", kind.Int),
		displayWidthBefore8019("int(11)"),
	),
	withVersionOverrides(
		mc("int_11", "INT(11)", fixt.IntZ, fixt.Int, "int", "int", kind.Int),
		displayWidthBefore8019("int(11)"),
	),
	withVersionOverrides(
		mc("int_24", "INT(24)", fixt.IntZ, fixt.Int, "int", "int", kind.Int),
		displayWidthBefore8019("int(24)"),
	),
	withVersionOverrides(
		mc("int_unsigned", "INT UNSIGNED", fixt.IntZ, int64(math.MaxUint32), "int", "int unsigned", kind.Int),
		displayWidthBefore8019("int(10) unsigned"),
	),
	withVersionOverrides(
		mc("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "bigint", "bigint", kind.Int),
		displayWidthBefore8019("bigint(20)"),
	),
	func() typetest.Case {
		// candidate issue: the NOT NULL column wraps values above
		// math.MaxInt64 to negative int64 (the maximum reads back as -1), and
		// querying the nullable column fails: the driver scans it as
		// sql.Null[uint64], which sq doesn't handle.
		c := mc("bigint_unsigned", "BIGINT UNSIGNED", fixt.IntZ, uint64(math.MaxUint64),
			"bigint", "bigint unsigned", kind.Int)
		c.VersionOverrides = displayWidthBefore8019("bigint(20) unsigned")
		c.PerVariant = typetest.PerVariant{
			NotNull:  typetest.Want{Val: typetest.Is[any](int64(-1))},
			Nullable: typetest.Want{QueryErr: typetest.Is("unknown type db(*sql.Null[uint64])")},
		}
		return c
	}(),
	func() typetest.Case {
		// candidate issue: BIT is reported as kind int, but its values come
		// back as a string of the raw bytes.
		c := mc("bit_1", "BIT(1)", fixt.IntZ, int64(1), "bit", "bit(1)", kind.Int)
		c.Want.Zero = typetest.Is[any]("\x00")
		c.Want.Val = typetest.Is[any]("\x01")
		return c
	}(),
	func() typetest.Case {
		// candidate issue: as for BIT(1).
		c := mc("bit_64", "BIT(64)", fixt.IntZ, fixt.Int, "bit", "bit(64)", kind.Int)
		c.Want.Zero = typetest.Is[any]("\x00\x00\x00\x00\x00\x00\x00\x00")
		c.Want.Val = typetest.Is[any]("\x00\x00\x00\x00\x00\x00\x00\x07")
		return c
	}(),
	func() typetest.Case {
		// DECIMAL is DECIMAL(10,0): MySQL rounds 77.77 to 78.
		c := mc("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "decimal", "decimal(10,0)", kind.Decimal)
		c.Want.Val = typetest.Is[any](decimal.New(78, 0))
		return c
	}(),
	func() typetest.Case {
		c := mc("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, decimal5_2Max, "decimal", "decimal(5,2)", kind.Decimal)
		c.Want.Zero = typetest.Is[any](decimal.New(0, -2))
		return c
	}(),
	mc("decimal_10", "DECIMAL(10)", fixt.DecimalZ, decimal10Max, "decimal", "decimal(10,0)", kind.Decimal),
	func() typetest.Case {
		// FLOAT is single precision. candidate issue: the NOT NULL column
		// returns the float32 value widened (7.699999809265137), while the
		// nullable column returns 7.7.
		c := mc("float", "FLOAT", fixt.FloatZ, fixt.Float, "float", "float", kind.Float)
		c.PerVariant.NotNull.Val = typetest.Is[any](float64(float32(fixt.Float)))
		return c
	}(),
	mc("double", "DOUBLE", fixt.FloatZ, fixt.Float, "double", "double", kind.Float),
	mc("char_255", "CHAR(255)", fixt.TextZ, fixt.Text, "char", "char(255)", kind.Text),
	mc("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "varchar(16)", kind.Text),
	mc("varchar_255", "VARCHAR(255)", fixt.TextZ, fixt.Text, "varchar", "varchar(255)", kind.Text),
	mc("tinytext", "TINYTEXT", fixt.TextZ, fixt.Text, "tinytext", "tinytext", kind.Text),
	mc("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text", kind.Text),
	mc("mediumtext", "MEDIUMTEXT", fixt.TextZ, fixt.Text, "mediumtext", "mediumtext", kind.Text),
	mc("longtext", "LONGTEXT", fixt.TextZ, fixt.Text, "longtext", "longtext", kind.Text),
	func() typetest.Case {
		// BINARY pads values to its length with zero bytes.
		c := mc("binary_64", "BINARY(64)", fixt.BytesZ, fixt.Bytes, "binary", "binary(64)", kind.Bytes)
		c.Want.Zero = typetest.Is[any](zeroPadded(fixt.BytesZ, 64))
		c.Want.Val = typetest.Is[any](zeroPadded(fixt.Bytes, 64))
		return c
	}(),
	mc("varbinary_64", "VARBINARY(64)", fixt.BytesZ, fixt.Bytes, "varbinary", "varbinary(64)", kind.Bytes),
	mc("tinyblob", "TINYBLOB", fixt.BytesZ, fixt.Bytes, "tinyblob", "tinyblob", kind.Bytes),
	mc("blob", "BLOB", fixt.BytesZ, fixt.Bytes, "blob", "blob", kind.Bytes),
	mc("mediumblob", "MEDIUMBLOB", fixt.BytesZ, fixt.Bytes, "mediumblob", "mediumblob", kind.Bytes),
	mc("longblob", "LONGBLOB", fixt.BytesZ, fixt.Bytes, "longblob", "longblob", kind.Bytes),
	mc("date", "DATE", fixt.DateZ, fixt.Date, "date", "date", kind.Date),
	mc("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "datetime", "datetime", kind.Datetime),
	mc("datetime_6", "DATETIME(6)", fixt.DatetimeZ, datetimeMicros, "datetime", "datetime(6)", kind.Datetime),
	func() typetest.Case {
		// With explicit_defaults_for_timestamp off (the 5.x images), the
		// first TIMESTAMP NOT NULL column in a table gets an implicit
		// default.
		c := mc("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "timestamp", "timestamp", kind.Datetime)
		c.VersionOverrides = []typetest.Override{{
			Before:     "v8.0.0",
			PerVariant: typetest.PerVariant{NotNull: typetest.Want{Default: typetest.Is("CURRENT_TIMESTAMP")}},
		}}
		return c
	}(),
	func() typetest.Case {
		// With explicit_defaults_for_timestamp off (the 5.x images), every
		// TIMESTAMP NOT NULL column after the first gets an implicit default
		// of '0000-00-00 00:00:00', which strict sql_mode (NO_ZERO_DATE)
		// rejects, so CREATE TABLE fails. col_timestamp is the first. This
		// can come back on 5.x once cases can declare defaults.
		c := mc("timestamp_6", "TIMESTAMP(6)", fixt.DatetimeZ, datetimeMicros, "timestamp", "timestamp(6)",
			kind.Datetime)
		c.MinVer = "v8.0.0"
		return c
	}(),
	mc("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time", kind.Time),
	func() typetest.Case {
		c := mc("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time(3)", kind.Time)
		c.Want.Zero = typetest.Is[any]("00:00:00.000")
		return c
	}(),
	func() typetest.Case {
		// MySQL reads a YEAR of 1 to 69 as 2001 to 2069.
		c := mc("year", "YEAR", fixt.IntZ, fixt.Int, "year", "year", kind.Int)
		c.Want.Val = typetest.Is[any](int64(2007))
		c.VersionOverrides = displayWidthBefore8019("year(4)")
		return c
	}(),
	mc("enum", "ENUM('alfa','bravo','charlie')", fixt.EnumAlfa, fixt.EnumBravo,
		"enum", "enum('alfa','bravo','charlie')", kind.Text),
	mc("set", "SET('alfa','bravo','charlie')", fixt.EnumAlfa, "alfa,bravo",
		"set", "set('alfa','bravo','charlie')", kind.Text),
	func() typetest.Case {
		c := mc("json", "JSON", fixt.JSONZ, fixt.JSON, "json", "json", kind.Text)
		c.MinVer = "v5.7.8"
		return c
	}(),
	func() typetest.Case {
		const pt = typetest.SQL("ST_GeomFromText('POINT(1 1)')")
		// candidate issue: spatial types have no kind.
		c := mc("geometry", "GEOMETRY", pt, pt, "geometry", "geometry", kind.Unknown)
		c.Want.Zero = typetest.Is[any](pointWKB)
		c.Want.Val = typetest.Is[any](pointWKB)
		return c
	}(),
	func() typetest.Case {
		const pt = typetest.SQL("ST_GeomFromText('POINT(1 1)')")
		// candidate issue: as for GEOMETRY.
		c := mc("point", "POINT", pt, pt, "point", "point", kind.Unknown)
		c.Want.Zero = typetest.Is[any](pointWKB)
		c.Want.Val = typetest.Is[any](pointWKB)
		return c
	}(),
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

// mc returns a MySQL case with the given inspect expectations.
func mc(name, decl string, zero, val any, baseType, colType string, k kind.Kind) typetest.Case {
	return typetest.Case{
		Name: name,
		Decl: decl,
		Zero: zero,
		Val:  val,
		Want: typetest.Want{
			BaseType:   typetest.Is(baseType),
			ColumnType: typetest.Is(colType),
			Kind:       typetest.Is(k),
		},
	}
}

var (
	datetimeMicros = time.Date(2017, 7, 7, 7, 7, 7, 123456000, time.UTC)
	decimal5_2Max  = decimal.RequireFromString("999.99")
	decimal10Max   = decimal.RequireFromString("9999999999")
)

// pointWKB is how a spatial column holding POINT(1 1) reads back today: a
// string of MySQL's internal format, a 4-byte SRID followed by the
// little-endian WKB.
const pointWKB = "\x00\x00\x00\x00" + // SRID 0
	"\x01\x01\x00\x00\x00" + // little-endian, type 1 (point)
	"\x00\x00\x00\x00\x00\x00\xf0?" + // x = 1.0
	"\x00\x00\x00\x00\x00\x00\xf0?" // y = 1.0

// displayWidthBefore8019 returns the override for an integer or YEAR column
// type on servers before 8.0.19, which still report the display width
// (int(11), year(4)). 8.0.19 stopped reporting it, except for TINYINT(1).
func displayWidthBefore8019(colType string) []typetest.Override {
	return []typetest.Override{{
		Before: "v8.0.19",
		Want:   typetest.Want{ColumnType: typetest.Is(colType)},
	}}
}

// withVersionOverrides returns c with c.VersionOverrides set to overrides.
// c.Want holds the expectations for the newest supported server. On an older
// server, the first override whose Before is newer than the server replaces
// the fields it sets. For example, the INT case expects column type "int",
// and displayWidthBefore8019("int(11)") adds an override that expects
// "int(11)" instead on servers older than 8.0.19.
//
// It lets a case built by mc take overrides inline in mysqlTypeCases, without
// a closure.
func withVersionOverrides(c typetest.Case, overrides []typetest.Override) typetest.Case {
	c.VersionOverrides = overrides
	return c
}

// zeroPadded returns b padded with zero bytes to length n.
func zeroPadded(b []byte, n int) []byte {
	out := make([]byte, n)
	copy(out, b)
	return out
}
