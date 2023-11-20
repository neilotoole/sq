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
	"text/template"
	"time"
	"unicode"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/alessio/shellescape"
	"github.com/google/uuid"
	"github.com/samber/lo"

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
	case "1", "yes", "Yes", "YES", "y", "Y", "on", "ON":
		return true, nil
	case "0", "no", "No", "NO", "n", "N", "off", "OFF":
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
// TODO: replace this usage with "github.com/c2h5oh/datasize",
// or maybe https://github.com/docker/go-units/.
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
// of a prefixed with prefix, unless a is nil, in which
// case nil is returned.
func PrefixSlice(a []string, prefix string) []string {
	if a == nil {
		return nil
	}
	if len(a) == 0 {
		return []string{}
	}
	ret := make([]string, len(a))
	sb := strings.Builder{}
	for i := 0; i < len(a); i++ {
		sb.Grow(len(a[i]) + len(prefix))
		sb.WriteString(prefix)
		sb.WriteString(a[i])
		ret[i] = sb.String()
		sb.Reset()
	}

	return ret
}

// SuffixSlice returns a new slice containing each element
// of a with suffix w. If a is nil, nil is returned.
func SuffixSlice(a []string, w string) []string {
	if a == nil {
		return nil
	}
	if len(a) == 0 {
		return []string{}
	}
	for i := range a {
		a[i] += w
	}
	return a
}

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
// This func is only tested with ASCII chars; results are not
// guaranteed for multibyte runes.
func TrimLen(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}

// TrimLenMiddle returns s but with a maximum length of maxLen,
// with the middle of s replaced with "...". If maxLen is a small
// number, the ellipsis may be shorter, e.g. a single char.
// This func is only tested with ASCII chars; results are not
// guaranteed for multibyte runes.
func TrimLenMiddle(s string, maxLen int) string {
	length := len(s)
	if maxLen <= 0 {
		return ""
	}
	if length <= maxLen {
		return s
	}

	switch maxLen {
	case 1:
		return s[0:1]
	case 2:
		return string(s[0]) + string(s[length-1])
	case 3:
		return string(s[0]) + "." + string(s[length-1])
	case 4:
		return string(s[0]) + ".." + string(s[length-1])
	case 5:
		return string(s[0]) + "..." + string(s[length-1])
	default:
	}

	trimLen := ((maxLen + 1) / 2) - 2
	return s[:trimLen] + "..." + s[len(s)-trimLen:]
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

// StripDoubleQuote strips double quotes from s,
// or returns s unchanged if it is not correctly double-quoted.
func StripDoubleQuote(s string) string {
	if len(s) < 2 {
		return s
	}

	if s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
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

// Strings returns a []string for a. If a is empty or nil,
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
//
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

// VisitLines visits the lines of s, returning a new string built from
// applying fn to each line.
func VisitLines(s string, fn func(i int, line string) string) string {
	var sb strings.Builder

	sc := bufio.NewScanner(strings.NewReader(s))
	var line string
	for i := 0; sc.Scan(); i++ {
		line = sc.Text()
		line = fn(i, line)
		if i > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(line)
	}

	return sb.String()
}

// IndentLines returns a new string built from indenting each line of s.
func IndentLines(s, indent string) string {
	return VisitLines(s, func(_ int, line string) string {
		return indent + line
	})
}

// HasAnyPrefix returns true if s has any of the prefixes.
func HasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// FilterPrefix returns a new slice containing each element
// of a that has prefix.
func FilterPrefix(prefix string, a ...string) []string {
	return lo.Filter(a, func(item string, index int) bool {
		return strings.HasPrefix(item, prefix)
	})
}

// ElementsHavingPrefix returns the elements of a that have prefix.
func ElementsHavingPrefix(a []string, prefix string) []string {
	return lo.Filter(a, func(item string, index int) bool {
		return strings.HasPrefix(item, prefix)
	})
}

// NewTemplate returns a new text template, with the sprig
// functions already loaded.
func NewTemplate(name, tpl string) (*template.Template, error) {
	t, err := template.New(name).Funcs(sprig.FuncMap()).Parse(tpl)
	if err != nil {
		return nil, errz.Err(err)
	}
	return t, nil
}

// ValidTemplate is a convenience wrapper around NewTemplate. It
// returns an error if the tpl is not a valid text template.
func ValidTemplate(name, tpl string) error {
	_, err := NewTemplate(name, tpl)
	return err
}

// ExecuteTemplate is a convenience function that constructs
// and executes a text template, returning the string value.
func ExecuteTemplate(name, tpl string, data any) (string, error) {
	t, err := NewTemplate(name, tpl)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err = t.Execute(buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ShellEscape escapes s, making it safe to pass to a shell.
// Note that empty string will be returned as two single quotes.
func ShellEscape(s string) string {
	return shellescape.Quote(s)
}
