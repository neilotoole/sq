// Package timez contains time functionality.
package timez

import (
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
)

const (
	// ISO8601 is (our definition of) the ISO8601 timestamp with millisecond
	// precision.
	ISO8601 = "2006-01-02T15:04:05.000Z07:00"

	// ISO8601Z is the same as ISO8601, but in zulu time.
	ISO8601Z = "2006-01-02T15:04:05.000Z"

	// RFC3339Variant is a variant using "-0700" suffix.
	RFC3339Variant = "2006-01-02T15:04:05-0700"

	// RFC3339Z is an RFC3339 format, in zulu time.
	RFC3339Z = "2006-01-02T15:04:05Z"

	// RFC3339NanoZ is time.RFC3339Nano in zulu time.
	RFC3339NanoZ = "2006-01-02T15:04:05.999999999Z"

	// ExcelLongDate is the "long date" used by Excel.
	ExcelLongDate = "Monday, January 2, 2006"

	// ExcelDatetimeMDYNoSeconds is a datetime format used by Excel.
	// The date part is MM/D/YY.
	ExcelDatetimeMDYNoSeconds = "01/2/06 15:04"

	// ExcelDatetimeMDYSeconds is similar to ExcelDatetimeMDYNoSeconds,
	// but includes a seconds component in the time.
	ExcelDatetimeMDYSeconds = "01/2/06 15:04:05"

	// DateHourMinuteSecond has date followed by time, including seconds.
	DateHourMinuteSecond = "2006-01-02 15:04:05"

	// DateHourMinute has date followed by time, not including seconds.
	DateHourMinute = "2006-01-02 15:04"
)

// TimestampUTC returns the ISO8601 representation of t in UTC.
func TimestampUTC(t time.Time) string {
	return t.UTC().Format(ISO8601)
}

// DateUTC returns a date representation (2020-10-31) of t in UTC.
func DateUTC(t time.Time) string {
	return t.UTC().Format(time.DateOnly)
}

// TimestampToRFC3339 takes a ISO8601, ISO8601_X or RFC3339
// timestamp, and returns RFC3339. That is, the milliseconds are dropped.
// On error, the empty string is returned.
func TimestampToRFC3339(s string) string {
	t, err := ParseTimestampUTC(s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(RFC3339Z)
}

// TimestampToDate takes a ISO8601, ISO8601_X or RFC3339
// timestamp, and returns just the date component.
// On error, the empty string is returned.
func TimestampToDate(s string) string {
	t, err := ParseTimestampUTC(s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.DateOnly)
}

// ParseTimestampUTC is the counterpart of TimestampUTC. It attempts
// to parse s first in ISO8601, then time.RFC3339 format, falling
// back to the subtly different variants.
func ParseTimestampUTC(s string) (time.Time, error) {
	t, err := time.Parse(ISO8601, s)
	if err == nil {
		return t.UTC(), nil
	}

	// Fallback to RFC3339
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t.UTC(), nil
	}

	t, err = time.Parse(RFC3339Variant, s)
	if err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, errz.Errorf("failed to parse timestamp {%s}", s)
}

// ParseLocalDate accepts a date string s, returning the local midnight
// time of that date. Arg s must in format "2006-01-02".
func ParseLocalDate(s string) (time.Time, error) {
	if !strings.ContainsRune(s, 'T') {
		// It's a date
		t, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return t, err
		}

		return t, nil
	}

	// There's a 'T' in s, which means it's probably a timestamp.
	return time.Time{}, errz.Errorf("invalid date format: %s", s)
}

// ParseDateUTC accepts a date string s, returning the UTC midnight
// time of that date. Arg s must in format "2006-01-02".
func ParseDateUTC(s string) (time.Time, error) {
	if !strings.ContainsRune(s, 'T') {
		// It's a date
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		if err != nil {
			return t, err
		}

		return t, nil
	}

	// There's a 'T' in s, which means it's probably a timestamp.
	return time.Time{}, errz.Errorf("invalid date format: %s", s)
}

// ParseDateOrTimestampUTC attempts to parse s as either
// a date (see ParseDateUTC), or timestamp (see ParseTimestampUTC).
// The returned time is in UTC.
func ParseDateOrTimestampUTC(s string) (time.Time, error) {
	if strings.ContainsRune(s, 'T') {
		return ParseTimestampUTC(s)
	}

	t, err := ParseDateUTC(s)
	return t.UTC(), err
}

// MustParse is like time.Parse, but panics on error.
func MustParse(layout, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}
	return t
}

// TimestampLayouts is a map of timestamp layout names to layout string.
var TimestampLayouts = map[string]string{
	"RFC3339":                   time.RFC3339,
	"RFC3339Z":                  RFC3339Z,
	"ISO8601":                   ISO8601,
	"ISO8601Z":                  ISO8601Z,
	"RFC3339Nano":               time.RFC3339Nano,
	"RFC3339NanoZ":              RFC3339NanoZ,
	"ANSIC":                     time.ANSIC,
	"UnixDate":                  time.UnixDate,
	"RubyDate":                  time.RubyDate,
	"RFC8222":                   time.RFC822,
	"RFC8222Z":                  time.RFC822Z,
	"RFC850":                    time.RFC850,
	"RFC1123":                   time.RFC1123,
	"RFC1123Z":                  time.RFC1123Z,
	"Stamp":                     time.Stamp,
	"StampMilli":                time.StampMilli,
	"StampMicro":                time.StampMicro,
	"StampNano":                 time.StampNano,
	"DateHourMinuteSecond":      DateHourMinuteSecond,
	"DateHourMinute":            DateHourMinute,
	"ExcelDatetimeMDYSeconds":   ExcelDatetimeMDYSeconds,
	"ExcelDatetimeMDYNoSeconds": ExcelDatetimeMDYNoSeconds,
}

var (
	LosAngeles = mustLoadLocation("America/Los_Angeles")
	Denver     = mustLoadLocation("America/Denver")
)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
