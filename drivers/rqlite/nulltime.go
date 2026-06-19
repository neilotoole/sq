package rqlite

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// rtypeNullTime is the reflect.Type of nullTime, used by setScanType for
// columns whose kind is Datetime or Date.
var rtypeNullTime = reflect.TypeFor[nullTime]()

// sqliteTimestampFormats is the timestamp-string parse set used by SQLite
// itself (verbatim from mattn/go-sqlite3.SQLiteTimestampFormats). rqlite
// returns SQLite cells as JSON over HTTP, so date/datetime values arrive
// here as plain strings rather than typed time.Time values, and we parse
// them with the same set SQLite uses. sqRows.Next (sqldriver.go)
// guarantees raw string delivery by bypassing gorqlite's two-format
// toTime pre-conversion, which would otherwise error out the whole
// result set for most of these formats (gh775).
var sqliteTimestampFormats = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// nullTime is the scan destination for rqlite columns mapped to a time
// kind. It accepts whatever shape gorqlite scans a JSON cell into
// (string, float64, int64, time.Time, or nil) and resolves it to a
// time.Time when possible. Genuinely unparseable text is preserved
// verbatim in String so the output writers can render it as-is.
type nullTime struct {
	Time   time.Time
	String string
	Valid  bool
	IsTime bool
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
		// gorqlite scans JSON numbers as float64. A column whose declared
		// kind is Datetime/Date but whose value comes through as a number
		// most often means SQLite REAL storage (e.g. a Julian day). We
		// don't interpret the value; we preserve its text form so the
		// query doesn't fail, mirroring the sqlite3 driver's float
		// handling (gh#471).
		*n = nullTime{String: strconv.FormatFloat(v, 'f', -1, 64), Valid: true}
		return nil
	default:
		return errz.Errorf("rqlite: cannot scan %T into nullTime", src)
	}
}

// scanText parses s using SQLite's standard timestamp formats. On
// success the value resolves to a time.Time; otherwise the raw text is
// preserved in String.
func (n *nullTime) scanText(s string) {
	trimmed := strings.TrimSuffix(s, "Z")
	for _, format := range sqliteTimestampFormats {
		if t, err := time.ParseInLocation(format, trimmed, time.UTC); err == nil {
			*n = nullTime{Time: t, Valid: true, IsTime: true}
			return
		}
	}
	*n = nullTime{String: s, Valid: true}
}

// epochToTime converts an integer epoch to a time.Time, mirroring the
// sqlite3 driver's heuristic: magnitudes above 1e12 are treated as
// milliseconds, otherwise as seconds.
func epochToTime(v int64) time.Time {
	if v > 1e12 || v < -1e12 {
		return time.Unix(0, v*int64(time.Millisecond)).UTC()
	}
	return time.Unix(v, 0).UTC()
}
