package timez

import (
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
)

const (
	// DateFormat is the layout for dates (without a time component), such as 2006-01-02.
	DateFormat = "2006-01-02"

	// TimeFormat is the layout for 24-hour time (without a date component), such as 15:04:05.
	TimeFormat = "15:04:05"

	// DatetimeFormat is the layout for a date/time timestamp.
	DatetimeFormat = time.RFC3339Nano
)

const (
	// RFC3339Milli is an RFC3339 format with millisecond precision.
	RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

	// RFC3339MilliZulu is the same as RFC3339Milli, but in zulu time.
	RFC3339MilliZulu = "2006-01-02T15:04:05.000Z"

	// rfc3339variant is a variant using "-0700" suffix.
	rfc3339variant = "2006-01-02T15:04:05-0700"

	// RFC3339Zulu is an RFC3339 format, in Zulu time.
	RFC3339Zulu = "2006-01-02T15:04:05Z"

	// ISO8601 is similar to RFC3339Milli, but doesn't have the colon
	// in the timezone offset.
	ISO8601 = "2006-01-02T15:04:05.000Z0700"

	// DateOnly is a date-only format.
	DateOnly = "2006-01-02"
)

// TimestampUTC returns the RFC3339Milli representation of t in UTC.
func TimestampUTC(t time.Time) string {
	return t.UTC().Format(RFC3339Milli)
}

// DateUTC returns a date representation (2020-10-31) of t in UTC.
func DateUTC(t time.Time) string {
	return t.UTC().Format(DateOnly)
}

// TimestampToRFC3339 takes a RFC3339Milli, ISO8601 or RFC3339
// timestamp, and returns RFC3339. That is, the milliseconds are dropped.
// On error, the empty string is returned.
func TimestampToRFC3339(s string) string {
	t, err := ParseTimestampUTC(s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(RFC3339Zulu)
}

// TimestampToDate takes a RFC3339Milli, ISO8601 or RFC3339
// timestamp, and returns just the date component.
// On error, the empty string is returned.
func TimestampToDate(s string) string {
	t, err := ParseTimestampUTC(s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(DateOnly)
}

// ParseTimestampUTC is the counterpart of TimestampUTC. It attempts
// to parse s first in RFC3339Milli, then time.RFC3339 format, falling
// back to the subtly different ISO8601 format.
func ParseTimestampUTC(s string) (time.Time, error) {
	t, err := time.Parse(RFC3339Milli, s)
	if err == nil {
		return t.UTC(), nil
	}

	// Fallback to RFC3339
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t.UTC(), nil
	}

	// Fallback to ISO8601
	t, err = time.Parse(ISO8601, s)
	if err == nil {
		return t.UTC(), nil
	}

	t, err = time.Parse(rfc3339variant, s)
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

	// There's a 'T' in s, which means its probably a timestamp.
	return time.Time{}, errz.Errorf("invalid date format: %s", s)
}

// ParseUTCDate accepts a date string s, returning the UTC midnight
// time of that date. Arg s must in format "2006-01-02".
func ParseUTCDate(s string) (time.Time, error) {
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
// a date (see ParseUTCDate), or timestamp (see ParseTimestampUTC).
// The returned time is in UTC.
func ParseDateOrTimestampUTC(s string) (time.Time, error) {
	if strings.ContainsRune(s, 'T') {
		return ParseTimestampUTC(s)
	}

	t, err := ParseUTCDate(s)
	return t.UTC(), err
}
