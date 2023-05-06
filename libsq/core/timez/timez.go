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
