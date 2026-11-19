package oracle_test

import (
	"context"
	"encoding/hex"
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

// TestColumnTypes checks what sq reports for each Oracle column type: the
// inspect fields, the query result kind, and the values read back. The
// expectations record current behavior, including values known to be wrong,
// which carry the issue that tracks them. See typetest and #1368.
func TestColumnTypes(t *testing.T) {
	t.Parallel()

	typetest.Run(t, oraConfig)
}

// TestColumnTypesDDL checks that testdata/coltypes.ddl matches the case
// table. See typetest.CheckDDL.
func TestColumnTypesDDL(t *testing.T) {
	typetest.CheckDDL(t, "testdata/coltypes.ddl", "Oracle 23ai", typetest.DDLTable{Name: "coltypes", Config: oraConfig})
}

// oraConfig is the typetest configuration for Oracle.
var oraConfig = typetest.Config{
	Handle:        sakila.Ora,
	RowIDDecl:     "NUMBER(10) NOT NULL PRIMARY KEY",
	FoldName:      strings.ToUpper,
	Cases:         oraTypeCases,
	CheckSettings: checkOraSettings,
	Literal:       oraLiteral,
}

// oraLiteral renders the inputs Oracle writes differently from
// typetest.DefaultLiteral: a time as a TIMESTAMP literal, since a plain string
// is read with the session's NLS_DATE_FORMAT, with an explicit UTC offset for
// the time zone types; and bytes with HEXTORAW.
func oraLiteral(col typetest.Column, v any) (string, bool) {
	switch v := v.(type) {
	case time.Time:
		ts := v.UTC().Format("2006-01-02 15:04:05.999999999")
		if strings.Contains(col.Case.Decl, "TIME ZONE") {
			ts += " +00:00"
		}
		return "TIMESTAMP '" + ts + "'", true
	case []byte:
		if v == nil {
			return "", false
		}
		return "HEXTORAW('" + hex.EncodeToString(v) + "')", true
	}
	return "", false
}

var (
	oraTimestampMillis = time.Date(2017, 7, 7, 7, 7, 7, 123000000, time.UTC)
	oraNumber5_2Max    = decimal.RequireFromString("999.99")
)

// oraTextZ is the zero input for the text types. Oracle treats an empty
// string as NULL, so "" can't go in a NOT NULL column; the varchar2_empty
// case records what happens to "".
const oraTextZ = "0"

// oraBytesZ is the zero input for the binary types, for the same reason.
var oraBytesZ = []byte{0}

var oraTypeCases = []typetest.Case{
	typetest.NewCase("number", "NUMBER", fixt.DecimalZ, fixt.Decimal, "NUMBER", "NUMBER", kind.Decimal),
	typetest.NewCase("number_10", "NUMBER(10)", fixt.IntZ, int64(math.MaxInt32), "NUMBER", "NUMBER(10)", kind.Int),
	typetest.NewCase("number_19", "NUMBER(19)", fixt.IntZ, int64(math.MaxInt64), "NUMBER", "NUMBER(19)", kind.Int),
	typetest.NewCase("number_5_2", "NUMBER(5,2)", fixt.DecimalZ, oraNumber5_2Max, "NUMBER", "NUMBER(5,2)",
		kind.Decimal),

	// INTEGER is stored as NUMBER(*,0): no precision, so sq types it as
	// decimal, as for bare NUMBER.
	typetest.NewCase("integer", "INTEGER", fixt.IntZ, int64(math.MaxInt32), "NUMBER", "NUMBER(*,0)",
		kind.Decimal).
		WithWantZero(decimal.New(0, 0)).
		WithWantVal(decimal.New(math.MaxInt32, 0)),

	// candidate issue: FLOAT is kind float in inspect, but its query result
	// kind is decimal, and its values come back as decimals.
	typetest.NewCase("float", "FLOAT", fixt.FloatZ, fixt.Float, "FLOAT", "FLOAT(126)", kind.Float).
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Decimal),
			Zero:      typetest.Is[any](decimal.New(0, 0)),
			Val:       typetest.Is[any](decimal.New(77, -1)),
		}),

	typetest.NewCase("binary_float", "BINARY_FLOAT", fixt.FloatZ, fixt.Float, "BINARY_FLOAT", "BINARY_FLOAT",
		kind.Float),
	typetest.NewCase("binary_double", "BINARY_DOUBLE", fixt.FloatZ, fixt.Float, "BINARY_DOUBLE", "BINARY_DOUBLE",
		kind.Float),

	// CHAR and NCHAR pad values to their length with spaces.
	typetest.NewCase("char_255", "CHAR(255)", oraTextZ, fixt.Text, "CHAR", "CHAR(255)", kind.Text).
		WithWantZero(typetest.SpacePadded(oraTextZ, 255)).
		WithWantVal(typetest.SpacePadded(fixt.Text, 255)),
	typetest.NewCase("char_10_char", "CHAR(10 CHAR)", oraTextZ, fixt.Text, "CHAR", "CHAR(10 CHAR)", kind.Text).
		WithWantZero(typetest.SpacePadded(oraTextZ, 10)).
		WithWantVal(typetest.SpacePadded(fixt.Text, 10)),
	typetest.NewCase("varchar2_16", "VARCHAR2(16)", oraTextZ, "0123456789abcdef", "VARCHAR2", "VARCHAR2(16)",
		kind.Text),
	typetest.NewCase("varchar2_16_char", "VARCHAR2(16 CHAR)", oraTextZ, "0123456789abcdef", "VARCHAR2",
		"VARCHAR2(16 CHAR)", kind.Text),
	typetest.NewCase("varchar2_4000", "VARCHAR2(4000)", oraTextZ, fixt.Text, "VARCHAR2", "VARCHAR2(4000)",
		kind.Text),

	// Oracle treats an empty string as NULL, so "" reads back as NULL. This
	// case has no NOT NULL variant, since "" can't be inserted into one.
	typetest.NewCase("varchar2_empty", "VARCHAR2(16)", "", fixt.Text, "VARCHAR2", "VARCHAR2(16)", kind.Text).
		WithVariants(typetest.Nullable).
		WithWantZero(nil),

	typetest.NewCase("nchar_16", "NCHAR(16)", oraTextZ, fixt.Text, "NCHAR", "NCHAR(16)", kind.Text).
		WithWantZero(typetest.SpacePadded(oraTextZ, 16)).
		WithWantVal(typetest.SpacePadded(fixt.Text, 16)),
	typetest.NewCase("nvarchar2_16", "NVARCHAR2(16)", oraTextZ, fixt.Text, "NVARCHAR2", "NVARCHAR2(16)",
		kind.Text),
	typetest.NewCase("clob", "CLOB", oraTextZ, fixt.Text, "CLOB", "CLOB", kind.Text),
	typetest.NewCase("nclob", "NCLOB", oraTextZ, fixt.Text, "NCLOB", "NCLOB", kind.Text),

	typetest.NewCase("raw_16", "RAW(16)", oraBytesZ, fixt.Bytes, "RAW", "RAW(16)", kind.Bytes),
	typetest.NewCase("blob", "BLOB", oraBytesZ, fixt.Bytes, "BLOB", "BLOB", kind.Bytes),

	typetest.NewCase("date", "DATE", fixt.DatetimeZ, fixt.Datetime, "DATE", "DATE", kind.Datetime),
	typetest.NewCase("timestamp", "TIMESTAMP", fixt.DatetimeZ, fixt.Datetime, "TIMESTAMP", "TIMESTAMP(6)",
		kind.Datetime),
	typetest.NewCase("timestamp_3", "TIMESTAMP(3)", fixt.DatetimeZ, oraTimestampMillis, "TIMESTAMP", "TIMESTAMP(3)",
		kind.Datetime),
	typetest.NewCase("timestamp_tz", "TIMESTAMP WITH TIME ZONE", fixt.DatetimeZ, fixt.Datetime,
		"TIMESTAMP WITH TIME ZONE", "TIMESTAMP(6) WITH TIME ZONE", kind.Datetime),

	// TIMESTAMP WITH LOCAL TIME ZONE values come back in a fixed zone named
	// for the database time zone, +00:00 (DBTIMEZONE, which checkOraSettings
	// pins).
	typetest.NewCase("timestamp_ltz", "TIMESTAMP WITH LOCAL TIME ZONE", fixt.DatetimeZ, fixt.Datetime,
		"TIMESTAMP WITH LOCAL TIME ZONE", "TIMESTAMP(6) WITH LOCAL TIME ZONE", kind.Datetime).
		WithWantZero(fixt.DatetimeZ.In(time.FixedZone("+00:00", 0))).
		WithWantVal(fixt.Datetime.In(time.FixedZone("+00:00", 0))),

	// INTERVAL values come back as Oracle's text form.
	typetest.NewCase("interval_ym", "INTERVAL YEAR TO MONTH", typetest.SQL("INTERVAL '0-0' YEAR TO MONTH"),
		typetest.SQL("INTERVAL '1-7' YEAR TO MONTH"), "INTERVAL YEAR TO MONTH", "INTERVAL YEAR(2) TO MONTH",
		kind.Text).
		WithWantZero("+00-00").
		WithWantVal("+01-07"),
	typetest.NewCase("interval_ds", "INTERVAL DAY TO SECOND", typetest.SQL("INTERVAL '0 00:00:00' DAY TO SECOND"),
		typetest.SQL("INTERVAL '7 07:07:07' DAY TO SECOND"), "INTERVAL DAY TO SECOND",
		"INTERVAL DAY(2) TO SECOND(6)", kind.Text).
		WithWantZero("+00 00:00:00.000000").
		WithWantVal("+07 07:07:07.000000"),

	// BOOLEAN and JSON have no kind in inspect. BOOLEAN values come back as
	// decimals 0 and 1, and JSON values as bytes.
	typetest.NewCase("boolean", "BOOLEAN", fixt.BoolZ, fixt.Bool, "BOOLEAN", "BOOLEAN", kind.Unknown).
		WithMinVer("v23.0.0").
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Decimal),
			Zero:      typetest.Is[any](decimal.New(0, 0)),
			Val:       typetest.Is[any](decimal.New(1, 0)),
		}).
		WithIssue("#1366"),
	typetest.NewCase("json", "JSON", fixt.JSONZ, fixt.JSON, "JSON", "JSON", kind.Unknown).
		WithMinVer("v21.0.0").
		WithWant(typetest.Want{
			QueryKind: typetest.Is(kind.Bytes),
			Zero:      typetest.Is[any]([]byte(`{}`)),
			Val:       typetest.Is[any]([]byte(`{"val":7}`)),
		}).
		WithIssue("#1366"),

	// Querying a VECTOR column fails: go-ora returns []float32, which sq
	// can't scan.
	typetest.NewCase("vector_3", "VECTOR(3, FLOAT32)", typetest.SQL("TO_VECTOR('[0,0,0]')"),
		typetest.SQL("TO_VECTOR('[1,2,3]')"), "VECTOR", "VECTOR(3,FLOAT32,DENSE)", kind.Unknown).
		WithMinVer("v23.4.0").
		WithWant(typetest.Want{
			QueryErr: typetest.Is("unsupported Scan, storing driver.Value type []float32"),
			Zero:     typetest.WantNull(),
			Val:      typetest.WantNull(),
		}).
		WithIssue("#1303"),
}

// oraSettings are the session settings the expectations in oraTypeCases
// assume: those of the sakiladb/oracle images.
type oraSettings struct {
	lengthSemantics string
	dbTimeZone      string
}

// checkOraSettings verifies that the settings are those the expectations
// assume. NLS_LENGTH_SEMANTICS decides whether a bare length such as
// VARCHAR2(16) means bytes or characters, and so how column_type reports it.
// DBTIMEZONE decides the zone that TIMESTAMP WITH LOCAL TIME ZONE values come
// back in: go-ora labels them with the database time zone, which it reads
// with SELECT DBTIMEZONE.
//
// The session time zone isn't checked: go-ora sets it from the local time
// zone of the machine running the test, so it's a property of the client,
// not the server. No expectation depends on it: go-ora binds a time.Time as
// TIMESTAMP WITH TIME ZONE, carrying its own zone, so inserts don't depend on
// the session zone either.
func checkOraSettings(ctx context.Context, db sqlz.DB, serverVer string) error {
	var got oraSettings
	err := db.QueryRowContext(ctx,
		"SELECT (SELECT value FROM nls_session_parameters WHERE parameter = 'NLS_LENGTH_SEMANTICS'), "+
			"DBTIMEZONE FROM dual").
		Scan(&got.lengthSemantics, &got.dbTimeZone)
	if err != nil {
		return errz.Err(err)
	}

	want := oraSettings{lengthSemantics: "BYTE", dbTimeZone: "+00:00"}
	if got != want {
		return errz.Errorf("Oracle %s: settings are %+v, expected %+v", serverVer, got, want)
	}
	return nil
}
