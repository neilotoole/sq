// Package stringz contains string functions similar in spirit
// to the stdlib strings package.
package stringz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unsafe"

	sprig "github.com/Masterminds/sprig/v3"
	"github.com/alessio/shellescape"
	"github.com/google/uuid"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/samber/lo"
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
		b[i] = charset[rand.Intn(len(charset))] //nolint:gosec
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

// TrimLen returns s but with a maximum length of maxLen.
// This func is only tested with ASCII chars; results are not
// guaranteed for multibyte runes.
func TrimLen(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen]
}

// Ellipsify shortens s to a length of maxLen by cutting the middle and
// inserting an ellipsis rune "…".This is the actual ellipsis rune, not
// three periods. For very short strings, the ellipsis may be elided.
//
// Be warned, Ellipsify may not be unicode-safe. Use at your own risk.
//
// See also: EllipsifyASCII.
func Ellipsify(s string, width int) string {
	const e = "…"
	if width <= 0 {
		return ""
	}
	length := len(s)

	if length <= width {
		return s
	}

	trimLen := ((width + 1) / 2) - 1
	return s[:trimLen+1-(width%2)] + e + s[len(s)-trimLen:]
}

// EllipsifyASCII returns s but with a maximum length of maxLen,
// with the middle of s replaced with "...". If maxLen is a small
// number, the ellipsis may be shorter, e.g. a single char.
// This func is only tested with ASCII chars; results are not
// guaranteed for multibyte runes.
//
// See also: Ellipsify.
func EllipsifyASCII(s string, width int) string {
	length := len(s)
	if width <= 0 {
		return ""
	}
	if length <= width {
		return s
	}

	switch width {
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

	trimLen := ((width + 1) / 2) - 2
	return s[:trimLen] + "..." + s[len(s)-trimLen:]
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
	if identRegex.MatchString(s) {
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

var filenameRegex = regexp.MustCompile(`[^a-zA-Z0-9-_ .(),+]`)

// SanitizeFilename returns a sanitized version of filename.
// The supplied value should be the base file name, not a path.
func SanitizeFilename(name string) string {
	const repl = "_"

	if name == "" {
		return ""
	}
	name = filenameRegex.ReplaceAllString(name, repl)
	if name == "" {
		return ""
	}

	name = filepath.Clean(name)
	// Some extra paranoid handling below.
	// Note that we know that filename is at least one char long.
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == ".":
		return strings.Replace(name, ".", repl, 1)
	case trimmed == "..":
		return strings.Replace(name, "..", repl+repl, 1)
	default:
		return name
	}
}

// TypeNames returns the go type of each element of a, as
// rendered by fmt "%T".
func TypeNames[T any](a ...T) []string {
	types := make([]string, len(a))
	for i := range a {
		types[i] = fmt.Sprintf("%T", a[i])
	}
	return types
}

// UnsafeBytes returns a byte slice for s without copying. This should really
// only be used when we're chasing nanoseconds, and [strings.Builder] isn't a
// good fit. UnsafeBytes uses package unsafe, and is generally sketchy.
func UnsafeBytes(s string) []byte {
	// https://josestg.medium.com/140x-faster-string-to-byte-and-byte-to-string-conversions-with-zero-allocation-in-go-200b4d7105fc
	p := unsafe.StringData(s)
	return unsafe.Slice(p, len(s))
}

// UnsafeString returns a string for b without copying. This should really only
// be used when we're chasing nanoseconds. UnsafeString uses package unsafe, and
// is generally sketchy.
func UnsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	// https://josestg.medium.com/140x-faster-string-to-byte-and-byte-to-string-conversions-with-zero-allocation-in-go-200b4d7105fc
	// Ignore if your IDE shows an error here; it's a false positive.
	p := unsafe.SliceData(b)
	return unsafe.String(p, len(b))
}
