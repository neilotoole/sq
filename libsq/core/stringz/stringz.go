// Package stringz contains string functions similar in spirit
// to the stdlib strings package.
package stringz

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// Redacted is the "xxxxx" string used for redacted
// values, such as passwords. We use "xxxxx" instead
// of the arguably prettier "*****" because stdlib
// uses this for redacted strings.
//
// See: url.URL.Redacted.
const Redacted = "xxxxx"

func init() { //nolint:gochecknoinits
	rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
}

// Reverse reverses the input string.
func Reverse(input string) string {
	n := 0
	runes := make([]rune, len(input))
	for _, r := range input {
		runes[n] = r
		n++
	}
	runes = runes[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		runes[i], runes[n-1-i] = runes[n-1-i], runes[i]
	}
	// Convert back to UTF-8.
	return string(runes)
}

// GenerateAlphaColName returns an Excel-style column name
// for index n, starting with A, B, C... and continuing
// to AA, AB, AC, etc...
func GenerateAlphaColName(n int, lower bool) string {
	start := 'A'
	if lower {
		start = 'a'
	}

	return genAlphaCol(n, start, 26)
}

func genAlphaCol(n int, start rune, lenAlpha int) string {
	buf := &bytes.Buffer{}
	for ; n >= 0; n = (n / lenAlpha) - 1 {
		buf.WriteRune(rune(n%lenAlpha) + start)
	}

	return Reverse(buf.String())
}

// ParseBool is an expansion of strconv.ParseBool that also
// accepts variants of "yes" and "no" (which are bool
// representations returned by some data sources).
func ParseBool(s string) (bool, error) {
	switch s {
	default:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return b, errz.Err(err)
		}
		return b, nil
	case "1", "yes", "Yes", "YES", "y", "Y":
		return true, nil
	case "0", "no", "No", "NO", "n", "N":
		return false, nil
	}
}

// InSlice returns true if the needle is present in the haystack.
func InSlice(haystack []string, needle string) bool {
	return SliceIndex(haystack, needle) != -1
}

// SliceIndex returns the index of needle in haystack, or -1.
func SliceIndex(haystack []string, needle string) int {
	for i, item := range haystack {
		if item == needle {
			return i
		}
	}
	return -1
}

// FormatFloat formats f. This method exists to provide a standard
// float formatting across the codebase.
func FormatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ByteSized returns a human-readable byte size, e.g. "2.1 MB", "3.0 TB", etc.
// TODO: replace this usage with "github.com/c2h5oh/datasize"
func ByteSized(size int64, precision int, sep string) string {
	f := float64(size)
	tpl := "%." + strconv.Itoa(precision) + "f" + sep

	switch {
	case f >= yb:
		return fmt.Sprintf(tpl+"YB", f/yb)
	case f >= zb:
		return fmt.Sprintf(tpl+"ZB", f/zb)
	case f >= eb:
		return fmt.Sprintf(tpl+"EB", f/eb)
	case f >= pb:
		return fmt.Sprintf(tpl+"PB", f/pb)
	case f >= tb:
		return fmt.Sprintf(tpl+"TB", f/tb)
	case f >= gb:
		return fmt.Sprintf(tpl+"GB", f/gb)
	case f >= mb:
		return fmt.Sprintf(tpl+"MB", f/mb)
	case f >= kb:
		return fmt.Sprintf(tpl+"KB", f/kb)
	}
	return fmt.Sprintf(tpl+"B", f)
}

const (
	_          = iota // ignore first value by assigning to blank identifier
	kb float64 = 1 << (10 * iota)
	mb
	gb
	tb
	pb
	eb
	zb
	yb
)

func SprintJSON(value any) string {
	j, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(j)
}

// UUID returns a new UUID string.
func UUID() string {
	return uuid.New().String()
}

// Uniq32 returns a UUID-like string that only contains
// alphanumeric chars. The result has length 32.
// The first element is guaranteed to be a letter.
func Uniq32() string {
	return UniqN(32)
}

// Uniq8 returns a UUID-like string that only contains
// alphanumeric chars. The result has length 8.
// The first element is guaranteed to be a letter.
func Uniq8() string {
	// I'm sure there's a more efficient way of doing this, but
	// this is fine for now.
	return UniqN(8)
}

// UniqSuffix returns s with a unique suffix.
func UniqSuffix(s string) string {
	return s + "_" + Uniq8()
}

// UniqPrefix returns s with a unique prefix.
func UniqPrefix(s string) string {
	return Uniq8() + "_" + s
}

const (
	// charsetAlphanumericLower is a set of characters to generate from. Note
	// that ambiguous chars such as "i" or "j" are excluded.
	charsetAlphanumericLower = "abcdefghkrstuvwxyz2345689"

	// charsetAlphaLower is similar to charsetAlphanumericLower, but
	// without numbers.
	charsetAlphaLower = "abcdefghkrstuvwxyz"
)

func stringWithCharset(length int, charset string) string {
	if charset == "" {
		panic("charset has zero length")
	}

	if length <= 0 {
		return ""
	}

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))] //#nosec G404 // Doesn't need to be strongly random
	}

	return string(b)
}

// UniqN returns a uniq string of length n. The first element is
// guaranteed to be a letter.
func UniqN(length int) string {
	switch {
	case length <= 0:
		return ""
	case length == 1:
		return stringWithCharset(1, charsetAlphaLower)
	default:
		return stringWithCharset(1, charsetAlphaLower) + stringWithCharset(length-1, charsetAlphanumericLower)
	}
}

// Plu handles the most common (English language) case of
// pluralization. With arg s being "row(s) col(s)", Plu
// returns "row col" if arg i is 1, otherwise returns "rows cols".
func Plu(s string, i int) string {
	if i == 1 {
		return strings.ReplaceAll(s, "(s)", "")
	}
	return strings.ReplaceAll(s, "(s)", "s")
}

// RepeatJoin returns a string consisting of count copies
// of s separated by sep. For example:
//
//	stringz.RepeatJoin("?", 3, ", ") == "?, ?, ?"
func RepeatJoin(s string, count int, sep string) string {
	if s == "" || count == 0 {
		return ""
	}
	if count == 1 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s)*count + len(sep)*(count-1))
	for i := 0; i < count; i++ {
		b.WriteString(s)
		if i < count-1 {
			b.WriteString(sep)
		}
	}

	return b.String()
}

// Surround returns s prefixed and suffixed with w.
func Surround(s, w string) string {
	sb := strings.Builder{}
	sb.Grow(len(s) + len(w)*2)
	sb.WriteString(w)
	sb.WriteString(s)
	sb.WriteString(w)
	return sb.String()
}

// SurroundSlice returns a new slice with each element
// of a prefixed and suffixed with w, unless a is nil,
// in which case nil is returned.
func SurroundSlice(a []string, w string) []string {
	if a == nil {
		return nil
	}
	if len(a) == 0 {
		return []string{}
	}
	ret := make([]string, len(a))
	sb := strings.Builder{}
	for i := 0; i < len(a); i++ {
		sb.Grow(len(a[i]) + len(w)*2)
		sb.WriteString(w)
		sb.WriteString(a[i])
		sb.WriteString(w)
		ret[i] = sb.String()
		sb.Reset()
	}

	return ret
}

// PrefixSlice returns a new slice with each element
// of a prefixed with w, unless a is nil, in which
// case nil is returned.
func PrefixSlice(a []string, w string) []string {
	if a == nil {
		return nil
	}
	if len(a) == 0 {
		return []string{}
	}
	ret := make([]string, len(a))
	sb := strings.Builder{}
	for i := 0; i < len(a); i++ {
		sb.Grow(len(a[i]) + len(w))
		sb.WriteString(w)
		sb.WriteString(a[i])
		ret[i] = sb.String()
		sb.Reset()
	}

	return ret
}

const (
	// DateFormat is the layout for dates (without a time component), such as 2006-01-02.
	DateFormat = "2006-01-02"

	// TimeFormat is the layout for 24-hour time (without a date component), such as 15:04:05.
	TimeFormat = "15:04:05"

	// DatetimeFormat is the layout for a date/time timestamp.
	DatetimeFormat = time.RFC3339Nano
)

// UniqTableName returns a new lower-case table name based on
// tbl, with a unique suffix, and a maximum length of 63. This
// value of 63 is chosen because it's less than the maximum table name
// length for Postgres, SQL Server, SQLite and MySQL.
func UniqTableName(tbl string) string {
	const maxLength = 63
	tbl = strings.TrimSpace(tbl)
	tbl = strings.ToLower(tbl)
	if tbl == "" {
		tbl = "tbl"
	}

	suffix := "__" + Uniq8()
	if len(tbl) > maxLength-len(suffix) {
		tbl = tbl[0 : maxLength-len(suffix)]
	}
	tbl += suffix

	// paranoid sanitization
	tbl = strings.ReplaceAll(tbl, "@", "_")
	tbl = strings.ReplaceAll(tbl, "/", "_")

	return tbl
}

// SanitizeAlphaNumeric replaces any non-alphanumeric
// runes of s with r (which is typically underscore).
//
//	a#2%3.4_ --> a_2_3_4_
func SanitizeAlphaNumeric(s string, r rune) string {
	runes := []rune(s)

	for i, v := range runes {
		switch {
		case v == r, unicode.IsLetter(v), unicode.IsNumber(v):
		default:
			runes[i] = r
		}
	}

	return string(runes)
}

// LineCount returns the number of lines in r. If skipEmpty is
// true, empty lines are skipped (a whitespace-only line is not
// considered empty). If r is nil or any error occurs, -1 is returned.
func LineCount(r io.Reader, skipEmpty bool) int {
	if r == nil {
		return -1
	}

	sc := bufio.NewScanner(r)
	var i int

	if skipEmpty {
		for sc.Scan() {
			if len(sc.Bytes()) > 0 {
				i++
			}
		}

		if sc.Err() != nil {
			return -1
		}

		return i
	}

	for i = 0; sc.Scan(); i++ { //nolint:revive
		// no-op
	}

	return i
}

// TrimLen returns s but with a maximum length of maxLen.
func TrimLen(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}

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

// DoubleQuote double-quotes (and escapes) s.
//
//	hello "world"  -->  "hello ""world"""
func DoubleQuote(s string) string {
	const q = '"'
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}

// BacktickQuote backtick-quotes (and escapes) s.
//
//	hello `world`  --> `hello ``world```
func BacktickQuote(s string) string {
	const q = '`'
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}

// SingleQuote single-quotes (and escapes) s.
//
//	jessie's girl  -->  'jessie''s girl'
func SingleQuote(s string) string {
	const q = '\''
	sb := strings.Builder{}
	sb.WriteRune(q)
	for _, r := range s {
		if r == q {
			sb.WriteRune(q)
		}
		sb.WriteRune(r)
	}
	sb.WriteRune(q)
	return sb.String()
}

// Type returns the printed type of v.
func Type(v any) string {
	return fmt.Sprintf("%T", v)
}

var identRegex = regexp.MustCompile(`\A[a-zA-Z][a-zA-Z0-9_]*$`)

// ValidIdent returns an error if s is not a valid identifier.
// And identifier must start with a letter, and may contain letters,
// numbers, and underscore.
func ValidIdent(s string) error {
	if identRegex.Match([]byte(s)) {
		return nil
	}

	return errz.Errorf("invalid identifier: %s", s)
}

// Strings returns a slice of [] for a. If a is empty or nil,
// an empty slice is returned. A nil element is treated as empty string.
func Strings[E any](a []E) []string {
	if len(a) == 0 {
		return []string{}
	}

	s := make([]string, len(a))
	for i := range a {
		switch v := any(a[i]).(type) {
		case nil:
			// nil is treated as empty string
		case string:
			s[i] = v
		case fmt.Stringer:
			s[i] = v.String()
		default:
			s[i] = fmt.Sprintf("%v", v)
		}
	}

	return s
}

// StringsD works like Strings, but it first dereferences
// every element of a. Thus if a is []any{*string, *int}, it
// is treated as if it were []any{string, int}.
func StringsD[E any](a []E) []string {
	if len(a) == 0 {
		return []string{}
	}

	s := make([]string, len(a))
	for i := range a {
		switch v := Val(a[i]).(type) {
		case nil:
			// nil is treated as empty string
		case string:
			s[i] = v
		case fmt.Stringer:
			s[i] = v.String()
		default:
			s[i] = fmt.Sprintf("%v", v)
		}
	}

	return s
}

// Val returns the fully dereferenced value of i. If i
// is nil, nil is returned. If i has type *(*string),
// Val(i) returns string.
// TODO: Should Val be renamed to Deref?
func Val(i any) any {
	if i == nil {
		return nil
	}

	v := reflect.ValueOf(i)
	for {
		if !v.IsValid() {
			return nil
		}

		switch v.Kind() { //nolint:exhaustive
		default:
			return v.Interface()
		case reflect.Ptr, reflect.Interface:
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
			// Loop again
			continue
		}
	}
}
