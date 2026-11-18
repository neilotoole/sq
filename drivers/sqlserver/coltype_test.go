package sqlserver_test

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/typetest"
)

// TestColumnTypes checks what sq reports for each SQL Server column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, typetest.Config{
		Handle:        sakila.MS,
		RowIDDecl:     "INT NOT NULL PRIMARY KEY",
		Cases:         msTypeCases,
		CheckSettings: checkMSSettings,
	})
}

var (
	msDatetimeMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	msDecimal5_2Max  = decimal.RequireFromString("999.99")
	msDecimal10Max   = decimal.RequireFromString("9999999999")
)

// msTypeCases are the SQL Server column-type cases. Issue "#1363" marks a
// column_type that doesn't match the declared type: SQL Server's column_type
// appends a precision or storage size (int(10), datetime(3)), reports MAX as
// -1 (varchar(-1)), and drops the scale (DECIMAL(5,2) as decimal(5)).
var msTypeCases = []typetest.Case{
	typetest.NewCase("bit", "BIT", fixt.BoolZ, fixt.Bool, "bit", "bit", kind.Bool),
	typetest.NewCase("tinyint", "TINYINT", fixt.IntZ, int64(math.MaxUint8), "tinyint", "tinyint(3)", kind.Int).
		WithIssue("#1363"),
	typetest.NewCase("smallint", "SMALLINT", fixt.IntZ, int64(math.MaxInt16), "smallint", "smallint(5)", kind.Int).
		WithIssue("#1363"),
	typetest.NewCase("int", "INT", fixt.IntZ, int64(math.MaxInt32), "int", "int(10)", kind.Int).
		WithIssue("#1363"),
	typetest.NewCase("bigint", "BIGINT", fixt.IntZ, int64(math.MaxInt64), "bigint", "bigint(19)", kind.Int).
		WithIssue("#1363"),

	// DECIMAL is DECIMAL(18,0): SQL Server rounds 77.77 to 78.
	typetest.NewCase("decimal", "DECIMAL", fixt.DecimalZ, fixt.Decimal, "decimal", "decimal(18)", kind.Decimal).
		WithWantVal(decimal.New(78, 0)).
		WithIssue("#1363"),
	typetest.NewCase("decimal_5_2", "DECIMAL(5,2)", fixt.DecimalZ, msDecimal5_2Max, "decimal", "decimal(5)",
		kind.Decimal).
		WithWantZero(decimal.New(0, -2)).
		WithIssue("#1363"),
	typetest.NewCase("decimal_10", "DECIMAL(10)", fixt.DecimalZ, msDecimal10Max, "decimal", "decimal(10)",
		kind.Decimal),
	typetest.NewCase("numeric", "NUMERIC", fixt.DecimalZ, fixt.Decimal, "numeric", "numeric(18)", kind.Decimal).
		WithWantVal(decimal.New(78, 0)).
		WithIssue("#1363"),

	// MONEY and SMALLMONEY have a scale of 4.
	typetest.NewCase("money", "MONEY", fixt.MoneyZ, fixt.Money, "money", "money(19)", kind.Decimal).
		WithWantZero(decimal.New(0, -4)).
		WithWantVal(decimal.New(777700, -4)).
		WithIssue("#1363"),
	typetest.NewCase("smallmoney", "SMALLMONEY", fixt.MoneyZ, fixt.Money, "smallmoney", "smallmoney(10)",
		kind.Decimal).
		WithWantZero(decimal.New(0, -4)).
		WithWantVal(decimal.New(777700, -4)).
		WithIssue("#1363"),

	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "float", "float(53)", kind.Float).
		WithIssue("#1363"),

	// FLOAT(24) is REAL, which is single precision.
	typetest.NewCase("float_24", "FLOAT(24)", fixt.FloatZ, fixt.Float, "real", "real(24)", kind.Float).
		WithWantVal(float64(float32(fixt.Float))).
		WithIssue("#1363"),
	typetest.NewCase("real", "REAL", fixt.FloatZ, fixt.Float, "real", "real(24)", kind.Float).
		WithWantVal(float64(float32(fixt.Float))).
		WithIssue("#1363"),

	typetest.NewCase("date", "DATE", fixt.DateZ, fixt.Date, "date", "date(0)", kind.Date).
		WithIssue("#1363"),
	typetest.NewCase("datetime", "DATETIME", fixt.DatetimeZ, fixt.Datetime, "datetime", "datetime(3)",
		kind.Datetime).
		WithIssue("#1363"),
	typetest.NewCase("datetime2", "DATETIME2", fixt.DatetimeZ, fixt.Datetime, "datetime2", "datetime2(7)",
		kind.Datetime).
		WithIssue("#1363"),
	typetest.NewCase("datetime2_3", "DATETIME2(3)", fixt.DatetimeZ, msDatetimeMillis, "datetime2", "datetime2(3)",
		kind.Datetime),

	// SMALLDATETIME has minute precision: 07:07:07 is stored as 07:07:00.
	typetest.NewCase("smalldatetime", "SMALLDATETIME", fixt.DatetimeZ, fixt.Datetime, "smalldatetime",
		"smalldatetime(0)", kind.Datetime).
		WithWantVal(fixt.Datetime.Truncate(time.Minute)).
		WithIssue("#1363"),

	// DATETIMEOFFSET values come back in a fixed zone with no name.
	typetest.NewCase("datetimeoffset", "DATETIMEOFFSET", fixt.DatetimeZ, fixt.Datetime, "datetimeoffset",
		"datetimeoffset(7)", kind.Datetime).
		WithWantZero(fixt.DatetimeZ.In(time.FixedZone("", 0))).
		WithWantVal(fixt.Datetime.In(time.FixedZone("", 0))).
		WithIssue("#1363"),

	// TIME values come back as a time.Time on 0001-01-01.
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time(7)", kind.Time).
		WithWantZero(timeOfDay(0, 0, 0, 0)).
		WithWantVal(timeOfDay(7, 7, 7, 0)).
		WithIssue("#1363"),
	typetest.NewCase("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time(3)", kind.Time).
		WithWantZero(timeOfDay(0, 0, 0, 0)).
		WithWantVal(timeOfDay(12, 34, 56, 789000000)),

	// CHAR and NCHAR pad values to their length with spaces.
	typetest.NewCase("char_255", "CHAR(255)", fixt.TextZ, fixt.Text, "char", "char(255)", kind.Text).
		WithWantZero(spacePadded(fixt.TextZ)).
		WithWantVal(spacePadded(fixt.Text)),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "varchar(16)",
		kind.Text),
	typetest.NewCase("varchar_max", "VARCHAR(MAX)", fixt.TextZ, fixt.Text, "varchar", "varchar(-1)", kind.Text).
		WithIssue("#1363"),
	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text(2147483647)", kind.Text).
		WithIssue("#1363"),
	typetest.NewCase("nchar_255", "NCHAR(255)", fixt.TextZ, fixt.Text, "nchar", "nchar(255)", kind.Text).
		WithWantZero(spacePadded(fixt.TextZ)).
		WithWantVal(spacePadded(fixt.Text)),
	typetest.NewCase("nvarchar_255", "NVARCHAR(255)", fixt.TextZ, fixt.Text, "nvarchar", "nvarchar(255)",
		kind.Text),
	typetest.NewCase("nvarchar_max", "NVARCHAR(MAX)", fixt.TextZ, fixt.Text, "nvarchar", "nvarchar(-1)",
		kind.Text).
		WithIssue("#1363"),
	typetest.NewCase("ntext", "NTEXT", fixt.TextZ, fixt.Text, "ntext", "ntext(1073741823)", kind.Text).
		WithIssue("#1363"),

	// go-mssqldb sends a Go nil as an nvarchar NULL, which binary columns
	// reject, so these bind []byte(nil) for NULL instead. See
	// Test_MSSQLDB_DriverIssue196. BINARY also pads values to its length
	// with zero bytes.
	typetest.NewCase("binary_255", "BINARY(255)", fixt.BytesZ, fixt.Bytes, "binary", "binary(255)", kind.Bytes).
		WithNullInput([]byte(nil)).
		WithWantZero(zeroPadded(fixt.BytesZ)).
		WithWantVal(zeroPadded(fixt.Bytes)),
	typetest.NewCase("varbinary_255", "VARBINARY(255)", fixt.BytesZ, fixt.Bytes, "varbinary", "varbinary(255)",
		kind.Bytes).
		WithNullInput([]byte(nil)),
	typetest.NewCase("varbinary_max", "VARBINARY(MAX)", fixt.BytesZ, fixt.Bytes, "varbinary", "varbinary(-1)",
		kind.Bytes).
		WithNullInput([]byte(nil)).
		WithIssue("#1363"),
	typetest.NewCase("image", "IMAGE", fixt.BytesZ, fixt.Bytes, "image", "image(2147483647)", kind.Bytes).
		WithNullInput([]byte(nil)).
		WithIssue("#1363"),

	// candidate issue: UNIQUEIDENTIFIER is kind text, but its values come back
	// as a string of the raw 16 bytes, not the GUID's text form.
	typetest.NewCase("uniqueidentifier", "UNIQUEIDENTIFIER", "00000000-0000-0000-0000-000000000000",
		"77777777-7777-7777-7777-777777777777", "uniqueidentifier", "uniqueidentifier", kind.Text).
		WithWantZero(strings.Repeat("\x00", 16)).
		WithWantVal(strings.Repeat("w", 16)),

	typetest.NewCase("xml", "XML", "", "<val>7</val>", "xml", "xml(-1)", kind.Text).
		WithIssue("#1363"),

	// candidate issue: the CLR types HIERARCHYID, GEOMETRY and GEOGRAPHY have
	// no kind, and their values come back as a string of their raw
	// serialized bytes.
	typetest.NewCase("hierarchyid", "HIERARCHYID", typetest.SQL("hierarchyid::Parse('/')"),
		typetest.SQL("hierarchyid::Parse('/1/')"), "hierarchyid", "hierarchyid(892)", kind.Unknown).
		WithWantZero("").
		WithWantVal("X").
		WithIssue("#1363"),
	typetest.NewCase("geometry", "GEOMETRY", typetest.SQL("geometry::STGeomFromText('POINT(1 1)', 0)"),
		typetest.SQL("geometry::STGeomFromText('POINT(1 1)', 0)"), "geometry", "geometry(-1)", kind.Unknown).
		WithWantZero(msPointSRID0).
		WithWantVal(msPointSRID0).
		WithIssue("#1363"),
	typetest.NewCase("geography", "GEOGRAPHY", typetest.SQL("geography::STGeomFromText('POINT(1 1)', 4326)"),
		typetest.SQL("geography::STGeomFromText('POINT(1 1)', 4326)"), "geography", "geography(-1)",
		kind.Unknown).
		WithWantZero(msPointSRID4326).
		WithWantVal(msPointSRID4326).
		WithIssue("#1363"),
}

// msPointSRID0 and msPointSRID4326 are how a GEOMETRY or GEOGRAPHY column
// holding POINT(1 1) reads back today: a string of SQL Server's serialized
// form, a 4-byte little-endian SRID, version 1, a point flag, then x and y.
const (
	msPointSuffix   = "\x01\x0c" + "\x00\x00\x00\x00\x00\x00\xf0?" + "\x00\x00\x00\x00\x00\x00\xf0?"
	msPointSRID0    = "\x00\x00\x00\x00" + msPointSuffix
	msPointSRID4326 = "\xe6\x10\x00\x00" + msPointSuffix
)

// timeOfDay returns the time.Time that a TIME column holding the given time
// of day reads back as: that time on 0001-01-01 UTC.
func timeOfDay(hour, minute, sec, nsec int) time.Time {
	return time.Date(1, time.January, 1, hour, minute, sec, nsec, time.UTC)
}

// fixedLen is the length of the fixed-length CHAR, NCHAR and BINARY cases.
const fixedLen = 255

// spacePadded returns s padded with spaces to fixedLen, as CHAR and NCHAR
// store it.
func spacePadded(s string) string {
	return s + strings.Repeat(" ", fixedLen-len(s))
}

// zeroPadded returns b padded with zero bytes to fixedLen, as BINARY stores
// it.
func zeroPadded(b []byte) []byte {
	out := make([]byte, fixedLen)
	copy(out, b)
	return out
}

// msSettings are the server settings the expectations in msTypeCases assume:
// those of the sakiladb/sqlserver images.
type msSettings struct {
	collation string
	language  string
}

// checkMSSettings verifies that the server's settings are those the
// expectations assume. The collation decides how text compares and pads, and
// the language how dates are parsed and formatted.
func checkMSSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got msSettings
	err := db.QueryRowContext(ctx,
		"SELECT CAST(DATABASEPROPERTYEX(DB_NAME(), 'Collation') AS NVARCHAR(128)), @@LANGUAGE").
		Scan(&got.collation, &got.language)
	if err != nil {
		return errz.Err(err)
	}

	want := msSettings{collation: "SQL_Latin1_General_CP1_CI_AS", language: "us_english"}
	if got != want {
		return errz.Errorf("SQL Server %s: settings are %+v, expected %+v", serverVer, got, want)
	}
	return nil
}
