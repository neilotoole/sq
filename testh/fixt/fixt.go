// Package fixt contains common test fixture values.
// It exists as its own package separate from testh mostly
// for semantic convenience. Broadly speaking, for any one
// database column, we want to test three values: the zero value,
// a non-zero value, and NULL (note that for some columns, e.g.
// primary key columns, NULL does not make sense). Thus for each
// value there is a ValueZ (zero) and Value (non-zero) item.
package fixt

import (
	"time"

	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/libsq/stringz"
)

// These consts are test fixtures for various data types.
const (
	Text       string  = "seven"
	TextZ      string  = ""
	Int        int64   = 7
	IntZ       int64   = 0
	Float      float64 = 7.7
	FloatZ     float64 = 0
	Decimal    string  = "77.77"
	DecimalZ   string  = "0"
	Money      string  = "77.77"
	MoneyZ     string  = "0.00"
	Bool       bool    = true
	BoolZ      bool    = false
	BitString  string  = "1001"
	BitStringZ string  = "0"
	TimeOfDay  string  = "07:07:07"
	TimeOfDayZ string  = "00:00:00"
	JSON       string  = `{"val": 7}`
	JSONZ      string  = "{}"
	EnumAlfa   string  = "alfa"
	EnumBravo  string  = "bravo"
	UUID       string  = "77777777-7777-7777-7777-777777777777"
	UUIDZ      string  = "00000000-0000-0000-0000-000000000000"
)

// These vars are text fixtures for various data types.
var (
	Bytes     = []byte("seven")
	BytesZ    = []byte("")
	Datetime  = mustParseTime(time.RFC3339, "2017-07-07T07:07:07-00:00").UTC()
	DatetimeZ = mustParseTime(time.RFC3339, "1989-11-09T00:00:00-00:00").UTC()
	Date      = mustParseTime(stringz.DateFormat, "2017-07-07").UTC()
	DateZ     = mustParseTime(stringz.DateFormat, "1989-11-09").UTC()
)

func mustParseTime(layout, value string) time.Time {
	t, err := time.ParseInLocation(layout, value, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

// ColNamePerKind returns a slice of column names, one column name for
// each kind (excepting KindUnknown and KindNull, unless withNull
// or withUnknown are set). If isIntBool is
// true, KindInt is returned for "col_bool", otherwise KindBool.
func ColNamePerKind(isIntBool bool, withNull bool, withUnknown bool) (colNames []string, kinds []sqlz.Kind) {
	colNames = []string{"col_int", "col_float", "col_decimal", "col_bool", "col_text", "col_datetime", "col_date", "col_time", "col_bytes"}
	kinds = []sqlz.Kind{sqlz.KindInt, sqlz.KindFloat, sqlz.KindDecimal, sqlz.KindBool, sqlz.KindText, sqlz.KindDatetime, sqlz.KindDate, sqlz.KindTime, sqlz.KindBytes}

	if isIntBool {
		kinds[3] = sqlz.KindInt
	}

	if withNull {
		colNames = append(colNames, "col_null")
		kinds = append(kinds, sqlz.KindNull)
	}
	if withUnknown {
		colNames = append(colNames, "col_unknown")
		kinds = append(kinds, sqlz.KindUnknown)
	}

	return colNames, kinds
}

// The gopher.gif image used for testing bytes.
const (
	GopherFilename = "gopher.gif"
	GopherPath     = "testh/fixt/testdata/gopher.gif"
	GopherSize     = 1788 // filesize in bytes
)
