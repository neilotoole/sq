// Package timefmt contains functionality for dealing with time formats.
package timefmt

import (
	"strconv"
	"strings"
	"time"

	"github.com/ncruces/go-strftime"
	"github.com/neilotoole/sq/libsq/core/timez"
)

// A set of common time layout formats.
const (
	ANSIC         = "ANSIC"
	DateOnly      = "DateOnly"
	DateTime      = "DateTime"
	ISO8601       = "ISO8601"
	RFC1123       = "RFC1123"
	RFC1123Z      = "RFC1123Z"
	RFC3339       = "RFC3339"
	RFC3339Milli  = "RFC3339Milli"
	RFC3339MilliZ = "RFC3339MilliZ"
	RFC3339Nano   = "RFC3339Nano"
	RFC3339Z      = "RFC3339Z"
	RFC822        = "RFC822"
	RFC822Z       = "RFC822Z"
	RFC850        = "RFC850"
	TimeOnly      = "TimeOnly"
	Unix          = "Unix"
	UnixDate      = "UnixDate"
	UnixMicro     = "UnixMicro"
	UnixMilli     = "UnixMilli"
	UnixNano      = "UnixNano"
)

var namedLayouts = []string{
	ANSIC,
	DateOnly,
	DateTime,
	ISO8601,
	RFC1123,
	RFC1123Z,
	RFC3339,
	RFC3339Milli,
	RFC3339MilliZ,
	RFC3339Nano,
	RFC3339Z,
	RFC822,
	RFC822Z,
	RFC850,
	TimeOnly,
	Unix,
	UnixDate,
	UnixMicro,
	UnixMilli,
	UnixNano,
}

var mUpperNamedStdlibLayouts = map[string]string{
	ANSIC:                          time.ANSIC,
	strings.ToUpper(DateOnly):      time.DateOnly,
	strings.ToUpper(DateTime):      time.DateTime,
	ISO8601:                        timez.ISO8601,
	RFC1123:                        time.RFC1123,
	RFC1123Z:                       time.RFC1123Z,
	RFC3339:                        time.RFC3339,
	strings.ToUpper(RFC3339Milli):  timez.RFC3339Milli,
	strings.ToUpper(RFC3339MilliZ): timez.RFC3339MilliZulu,
	strings.ToUpper(RFC3339Nano):   time.RFC3339Nano,
	strings.ToUpper(RFC3339Z):      timez.RFC3339Zulu,
	RFC822:                         time.RFC822,
	RFC822Z:                        time.RFC822Z,
	RFC850:                         time.RFC850,
	strings.ToUpper(TimeOnly):      time.TimeOnly,
	strings.ToUpper(UnixDate):      time.UnixDate,
}

func unix(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func unixMilli(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func unixMicro(t time.Time) string {
	return strconv.FormatInt(t.UnixMicro(), 10)
}

func unixNano(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), 10)
}

// NamedLayouts returns a slice containing the layout names
// supported by FormatFunc.
func NamedLayouts() []string {
	a := make([]string, len(namedLayouts))
	copy(a, namedLayouts)
	return a
}

// FormatFunc returns a time format function. If layout is a named
// layout (per NamedLayouts, ignoring case), a func for the named
// layout is returned. Otherwise layout is treated as a strftime layout
// (NOT a stdlib time layout).
func FormatFunc(layout string) func(time.Time) string {
	lu := strings.ToUpper(layout)
	switch lu {
	// Special handling for the unix times, because it seems it's not
	// possible to express unix time in stdlib layout format?
	case "UNIX":
		return unix
	case "UNIXMILLI":
		return unixMilli
	case "UNIXMICRO":
		return unixMicro
	case "UNIXNANO":
		return unixNano
	}

	var f string
	var ok bool

	if f, ok = mUpperNamedStdlibLayouts[lu]; ok {
		return func(t time.Time) string {
			return t.Format(f)
		}
	}

	// It's not a named layout, use strftime
	return func(t time.Time) string {
		return strftime.Format(layout, t)
	}
}
