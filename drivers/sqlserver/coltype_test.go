package sqlserver_test

import (
	"bytes"
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
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

// TestColumnTypes_ROWVERSION covers ROWVERSION, which typetest.Run can't: a
// table can have only one ROWVERSION column, its values can't be inserted
// (SQL Server generates them), and they depend on database activity. Like
// TestColumnTypes, it records current behavior. Since the values aren't
// known in advance, it checks their properties: 8 bytes, different per row,
// and changed by an UPDATE.
func TestColumnTypes_ROWVERSION(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.MS) {
		t.Skipf("source %s not configured", sakila.MS)
	}
	src := th.Source(sakila.MS)
	db := th.OpenDB(src)

	serverVer, err := th.Open(src).DBSemver(th.Context)
	require.NoError(t, err)
	t.Logf("%s: server version %s", sakila.MS, serverVer)

	tbl := stringz.UniqTableName("rowversion")
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (row_id INT NOT NULL PRIMARY KEY, col_val INT NULL, col_rowversion ROWVERSION)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })
	_, err = db.ExecContext(th.Context, "INSERT INTO "+tbl+" (row_id) VALUES (1), (2)")
	require.NoError(t, err)

	// INFORMATION_SCHEMA reports ROWVERSION by its old name, timestamp.
	// candidate issue: it's kind int in inspect, but its query result kind is
	// bytes, and its values come back as 8 raw bytes.
	want := typetest.Column{
		Case:       &typetest.Case{Name: "rowversion", Decl: "ROWVERSION"},
		Name:       "col_rowversion",
		BaseType:   "timestamp",
		ColumnType: "timestamp",
		Kind:       kind.Int,
	}

	t.Run("TableMetadata", func(t *testing.T) {
		t.Parallel()
		tblMeta, err := th.TableMetadata(src, tbl)
		require.NoError(t, err)
		col := tblMeta.Column(want.Name)
		require.NotNil(t, col)
		require.Empty(t, typetest.DiffMeta("TableMetadata", serverVer, col, want))
	})

	t.Run("SourceMetadata", func(t *testing.T) {
		t.Parallel()
		srcMeta, err := th.SourceMetadata(src)
		require.NoError(t, err)
		tblMeta := srcMeta.Table(tbl)
		require.NotNil(t, tblMeta)
		col := tblMeta.Column(want.Name)
		require.NotNil(t, col)
		require.Empty(t, typetest.DiffMeta("SourceMetadata", serverVer, col, want))
	})

	query := "SELECT row_id, col_rowversion FROM " + tbl + " ORDER BY row_id"
	sink, err := th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	require.Len(t, sink.Recs, 2)
	require.Equal(t, kind.Bytes.String(), sink.RecMeta[1].Kind().String(), "query result kind")

	before := make([][]byte, 2)
	for i, rec := range sink.Recs {
		v, ok := rec[1].([]byte)
		require.True(t, ok, "row %d: want []byte, got %T", i+1, rec[1])
		require.Len(t, v, 8, "row %d", i+1)
		before[i] = v
	}
	require.NotEqual(t, before[0], before[1], "each row has its own rowversion")

	// Updating a row gives it a new rowversion, greater than any before.
	_, err = db.ExecContext(th.Context, "UPDATE "+tbl+" SET col_val = 7 WHERE row_id = 1")
	require.NoError(t, err)
	sink, err = th.QuerySQL(src, nil, query)
	require.NoError(t, err)
	after, ok := sink.Recs[0][1].([]byte)
	require.True(t, ok)
	require.Positive(t, bytes.Compare(after, before[0]), "the updated row's rowversion increased")
	require.Positive(t, bytes.Compare(after, before[1]), "and is greater than the other row's")
	require.Equal(t, before[1], sink.Recs[1][1], "the other row's rowversion is unchanged")
}

var (
	msDatetimeMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	msDecimal5_2Max  = decimal.RequireFromString("999.99")
	msDecimal10Max   = decimal.RequireFromString("9999999999")
)

// msTypeCases are the SQL Server column-type cases. Issue "#1363" marks a
// column_type that #1363 says is wrong: a precision or storage size appended
// to a type that can't declare one (int(10), datetime(3), text(2147483647)),
// MAX reported as -1 (varchar(-1)), or a scale left out (DECIMAL(5,2) as
// decimal(5), and DECIMAL(10) as decimal(10) where #1363 expects
// decimal(10,0)). A default precision on a type that can declare one
// (datetime2(7), time(7), datetimeoffset(7), float(53)) is correct, so those
// cases aren't marked. ROWVERSION isn't here: see TestColumnTypes_ROWVERSION.
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
		kind.Decimal).
		WithIssue("#1363"),
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

	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "float", "float(53)", kind.Float),

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
		kind.Datetime),
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
		WithWantVal(fixt.Datetime.In(time.FixedZone("", 0))),

	// TIME values come back as a time.Time on 0001-01-01.
	typetest.NewCase("time", "TIME", fixt.TimeOfDayZ, fixt.TimeOfDay, "time", "time(7)", kind.Time).
		WithWantZero(typetest.TimeOfDay(0, 0, 0, 0)).
		WithWantVal(typetest.TimeOfDay(7, 7, 7, 0)),
	typetest.NewCase("time_3", "TIME(3)", fixt.TimeOfDayZ, "12:34:56.789", "time", "time(3)", kind.Time).
		WithWantZero(typetest.TimeOfDay(0, 0, 0, 0)).
		WithWantVal(typetest.TimeOfDay(12, 34, 56, 789000000)),

	// CHAR and NCHAR pad values to their length with spaces.
	typetest.NewCase("char_255", "CHAR(255)", fixt.TextZ, fixt.Text, "char", "char(255)", kind.Text).
		WithWantZero(typetest.SpacePadded(fixt.TextZ, msFixedLen)).
		WithWantVal(typetest.SpacePadded(fixt.Text, msFixedLen)),
	typetest.NewCase("varchar_16", "VARCHAR(16)", fixt.TextZ, "0123456789abcdef", "varchar", "varchar(16)",
		kind.Text),
	typetest.NewCase("varchar_max", "VARCHAR(MAX)", fixt.TextZ, fixt.Text, "varchar", "varchar(-1)", kind.Text).
		WithIssue("#1363"),
	typetest.NewCase("text", "TEXT", fixt.TextZ, fixt.Text, "text", "text(2147483647)", kind.Text).
		WithIssue("#1363"),
	typetest.NewCase("nchar_255", "NCHAR(255)", fixt.TextZ, fixt.Text, "nchar", "nchar(255)", kind.Text).
		WithWantZero(typetest.SpacePadded(fixt.TextZ, msFixedLen)).
		WithWantVal(typetest.SpacePadded(fixt.Text, msFixedLen)),
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
		WithWantZero(typetest.ZeroPadded(fixt.BytesZ, msFixedLen)).
		WithWantVal(typetest.ZeroPadded(fixt.Bytes, msFixedLen)),
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

	// SQL_VARIANT holds values of other types, each with its own base type.
	// The Zero input is an integer and the Val input a string.
	//
	// candidate issue: the integer reads back as int64 from the NOT NULL
	// column, but as the string "0" from the nullable column.
	typetest.NewCase("sql_variant", "SQL_VARIANT", fixt.IntZ, fixt.Text, "sql_variant", "sql_variant(0)",
		kind.Unknown).
		WithPerVariant(typetest.PerVariant{Nullable: typetest.Want{Zero: typetest.Is[any]("0")}}).
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
		typetest.SQL("geometry::STGeomFromText('POINT(2 2)', 0)"), "geometry", "geometry(-1)", kind.Unknown).
		WithWantZero(msSRID0 + msPoint11).
		WithWantVal(msSRID0 + msPoint22).
		WithIssue("#1363"),
	typetest.NewCase("geography", "GEOGRAPHY", typetest.SQL("geography::STGeomFromText('POINT(1 1)', 4326)"),
		typetest.SQL("geography::STGeomFromText('POINT(2 2)', 4326)"), "geography", "geography(-1)",
		kind.Unknown).
		WithWantZero(msSRID4326 + msPoint11).
		WithWantVal(msSRID4326 + msPoint22).
		WithIssue("#1363"),
}

// These are the pieces of how a GEOMETRY or GEOGRAPHY column holding a point
// reads back today: a string of SQL Server's serialized form, a 4-byte
// little-endian SRID (msSRID0, msSRID4326), then version 1, a point flag, and
// x and y as little-endian float64 (msPoint11 is POINT(1 1), msPoint22 is
// POINT(2 2)).
const (
	msSRID0    = "\x00\x00\x00\x00"
	msSRID4326 = "\xe6\x10\x00\x00"
	msPoint11  = "\x01\x0c" + "\x00\x00\x00\x00\x00\x00\xf0?" + "\x00\x00\x00\x00\x00\x00\xf0?"
	msPoint22  = "\x01\x0c" + "\x00\x00\x00\x00\x00\x00\x00@" + "\x00\x00\x00\x00\x00\x00\x00@"
)

// msFixedLen is the length of the fixed-length CHAR, NCHAR and BINARY
// cases, which pad their values to it.
const msFixedLen = 255

// msSettings are the server settings the expectations in msTypeCases assume:
// those of the sakiladb/sqlserver images.
type msSettings struct {
	collation   string
	language    string
	ansiPadding int
}

// checkMSSettings verifies that the server's settings are those the
// expectations assume. The collation decides how text compares, ANSI_PADDING
// whether CHAR and BINARY values are padded to their length (with it off, a
// nullable CHAR column behaves like VARCHAR), and the language how dates are
// parsed and formatted.
func checkMSSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got msSettings
	err := db.QueryRowContext(ctx,
		"SELECT CAST(DATABASEPROPERTYEX(DB_NAME(), 'Collation') AS NVARCHAR(128)), @@LANGUAGE, "+
			"CAST(SESSIONPROPERTY('ANSI_PADDING') AS INT)").
		Scan(&got.collation, &got.language, &got.ansiPadding)
	if err != nil {
		return errz.Err(err)
	}

	want := msSettings{collation: "SQL_Latin1_General_CP1_CI_AS", language: "us_english", ansiPadding: 1}
	if got != want {
		return errz.Errorf("SQL Server %s: settings are %+v, expected %+v", serverVer, got, want)
	}
	return nil
}
