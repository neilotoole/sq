package timez

import (
	"strconv"
	"strings"
	"time"

	strftime "github.com/ncruces/go-strftime"
)

const (
	// DefaultDatetime is the name of the default datetime layout.
	// Its value is NOT the actual layout value itself.
	// Use FormatFunc(DefaultDatetime) to get a format function.
	DefaultDatetime = rfc3339Z

	// DefaultDate is the name of the default date layout.
	// Its value is NOT the actual layout value itself.
	// Use FormatFunc(DefaultDate) to get a format function.
	DefaultDate = dateOnly

	// DefaultTime is the name of the default time layout.
	// Its value is NOT the actual layout value itself.
	// Use FormatFunc(DefaultTime) to get a format function.
	DefaultTime = timeOnly
)

// A set common layout format names.
const (
	ansic       = "ANSIC"
	dateOnly    = "DateOnly"
	dateTime    = "DateTime"
	iso8601     = "ISO8601"
	iso8601z    = "ISO8601Z"
	rfc1123     = "RFC1123"
	rfc1123Z    = "RFC1123Z"
	rfc3339     = "RFC3339"
	rfc3339Nano = "RFC3339Nano"
	rfc3339Z    = "RFC3339Z"
	rfc822      = "RFC822"
	rfc822Z     = "RFC822Z"
	rfc850      = "RFC850"
	timeOnly    = "TimeOnly"
	unix        = "Unix"
	unixDate    = "UnixDate"
	unixMicro   = "UnixMicro"
	unixMilli   = "UnixMilli"
	unixNano    = "UnixNano"
)

var namedLayouts = []string{
	ansic,
	dateOnly,
	dateTime,
	iso8601,
	iso8601z,
	rfc1123,
	rfc1123Z,
	rfc3339,
	rfc3339Nano,
	rfc3339Z,
	rfc822,
	rfc822Z,
	rfc850,
	timeOnly,
	unix,
	unixDate,
	unixMicro,
	unixMilli,
	unixNano,
}

// mNamedStdlibLayouts is a map of uppercase layout name
// to the stdlib layout format.
var mNamedStdlibLayouts = map[string]string{
	ansic:                        time.ANSIC,
	strings.ToUpper(dateOnly):    time.DateOnly,
	strings.ToUpper(dateTime):    time.DateTime,
	iso8601:                      ISO8601,
	iso8601z:                     ISO8601Z,
	rfc1123:                      time.RFC1123,
	rfc1123Z:                     time.RFC1123Z,
	rfc3339:                      time.RFC3339,
	strings.ToUpper(rfc3339Nano): time.RFC3339Nano,
	rfc3339Z:                     RFC3339Z,
	rfc822:                       time.RFC822,
	rfc822Z:                      time.RFC822Z,
	rfc850:                       time.RFC850,
	strings.ToUpper(timeOnly):    time.TimeOnly,
	strings.ToUpper(unixDate):    time.UnixDate,
}

func doUnix(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func doUnixMilli(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}

func doUnixMicro(t time.Time) string {
	return strconv.FormatInt(t.UnixMicro(), 10)
}

func doUnixNano(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), 10)
}

// NamedLayouts returns a new slice containing the layout names
// supported by FormatFunc.
func NamedLayouts() []string {
	a := make([]string, len(namedLayouts))
	copy(a, namedLayouts)
	return a
}

// FormatFunc returns a time format function. If layout is a named
// layout (per NamedLayouts, ignoring case), a func for the named
// layout is returned. Otherwise layout is treated as a strftime layout
// (NOT as a stdlib time layout).
func FormatFunc(layout string) func(time.Time) string {
	lu := strings.ToUpper(layout)
	switch lu {
	// Special handling for the unix times, because it seems it's not
	// possible to express unix time in stdlib layout format.
	case strings.ToUpper(unix):
		return doUnix
	case strings.ToUpper(unixMilli):
		return doUnixMilli
	case strings.ToUpper(unixMicro):
		return doUnixMicro
	case strings.ToUpper(unixNano):
		return doUnixNano
	}

	if f, ok := mNamedStdlibLayouts[lu]; ok {
		return func(t time.Time) string {
			return t.Format(f)
		}
	}

	// It's not a named layout, use strftime
	return func(t time.Time) string {
		return strftime.Format(layout, t)
	}
}
