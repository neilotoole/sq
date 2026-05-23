package sqlite3

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	mattn "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// rtypeNullTime is the reflect.Type of nullTime. setScanType uses it as the
// scan destination for SQLite columns mapped to kind.Datetime or kind.Date.
var rtypeNullTime = reflect.TypeFor[nullTime]()

// nullTime is the scan destination for SQLite columns mapped to a time kind
// (kind.Datetime, kind.Date). It exists because mattn/go-sqlite3 only converts
// a cell to time.Time when the column's declared type is exactly "date",
// "datetime", or "timestamp" — it does not strip a parameterized suffix such
// as "datetime(6)" (as written by Rails). For such columns mattn returns the
// raw string or int instead, which cannot scan into *sql.NullTime, producing:
//
//	unsupported Scan, storing driver.Value type string into type *time.Time
//
// nullTime accepts whatever mattn returns and, for string/[]byte values,
// re-runs mattn's own SQLiteTimestampFormats — i.e. the parse mattn would have
// performed had it stripped the parens — recovering a proper time.Time.
// Genuinely unparseable text is preserved verbatim in String.
type nullTime struct {
	Time   time.Time
	String string
	Valid  bool // a non-NULL value was scanned
	IsTime bool // value resolved to a time.Time (else use String)
}

// Scan implements sql.Scanner.
func (n *nullTime) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*n = nullTime{}
		return nil
	case time.Time:
		*n = nullTime{Time: v, Valid: true, IsTime: true}
		return nil
	case string:
		n.scanText(v)
		return nil
	case []byte:
		n.scanText(string(v))
		return nil
	case int64:
		*n = nullTime{Time: epochToTime(v), Valid: true, IsTime: true}
		return nil
	case float64:
		// SQLite REAL storage (e.g. a Julian day) for a time column. mattn
		// returns these as float64 with no conversion; we don't try to
		// interpret them as instants, but preserve the value as text so the
		// query doesn't hard-fail (the goal of #471).
		*n = nullTime{String: strconv.FormatFloat(v, 'f', -1, 64), Valid: true}
		return nil
	default:
		return errz.Errorf("sqlite3: cannot scan %T into nullTime", src)
	}
}

// scanText parses s using mattn's SQLiteTimestampFormats, the same set mattn
// applies to exact-match date/datetime/timestamp columns. On success the value
// resolves to a time.Time; otherwise the raw text is preserved in String.
func (n *nullTime) scanText(s string) {
	trimmed := strings.TrimSuffix(s, "Z")
	for _, format := range mattn.SQLiteTimestampFormats {
		if t, err := time.ParseInLocation(format, trimmed, time.UTC); err == nil {
			*n = nullTime{Time: t, Valid: true, IsTime: true}
			return
		}
	}
	*n = nullTime{String: s, Valid: true}
}

// epochToTime converts an integer epoch to a time.Time, mirroring
// mattn/go-sqlite3's heuristic: magnitudes above 1e12 are treated as
// milliseconds, otherwise as seconds.
func epochToTime(v int64) time.Time {
	if v > 1e12 || v < -1e12 {
		return time.Unix(0, v*int64(time.Millisecond)).UTC()
	}
	return time.Unix(v, 0).UTC()
}
