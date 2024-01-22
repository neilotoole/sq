// Package tu contains basic generic test utilities.
package tu

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// SkipIff skips t if b is true. If msgAndArgs is non-empty, its first
// element must be a string, which can be a format string if there are
// additional elements.
//
// Examples:
//
//	tu.SkipIff(t, a == b)
//	tu.SkipIff(t, a == b, "skipping because a == b")
//	tu.SkipIff(t, a == b, "skipping because a is %v and b is %v", a, b)
func SkipIff(t testing.TB, b bool, format string, args ...any) {
	if b {
		if format == "" {
			t.SkipNow()
		} else {
			t.Skipf(format, args...)
		}
	}
}

// StructFieldValue extracts the value of fieldName from arg strct.
// If strct is nil, nil is returned.
// The function will panic if strct is not a struct (or pointer to struct), or if
// the struct does not have fieldName. The returned value may be nil if the
// field is a pointer and is nil.
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: SliceFieldValues, SliceFieldKeyValues.
func StructFieldValue(fieldName string, strct any) any {
	if strct == nil {
		return nil
	}

	// zv is the zero value of reflect.Value, which can be returned by FieldByName
	zv := reflect.Value{}

	e := reflect.Indirect(reflect.ValueOf(strct))
	if e.Kind() != reflect.Struct {
		panic(fmt.Sprintf("strct expected to be struct but was %s", e.Kind()))
	}

	f := e.FieldByName(fieldName)
	if f == zv { //nolint:govet
		// According to govet:
		//
		//   reflectvaluecompare: avoid using == with reflect.Value
		//
		// Maybe we should be using f.IsZero instead?

		panic(fmt.Sprintf("struct (%T) does not have field {%s}", strct, fieldName))
	}
	fieldValue := f.Interface()
	return fieldValue
}

// SliceFieldValues takes a slice of structs, and returns a slice
// containing the value of fieldName for each element of slice.
//
// Note that slice can be []interface{}, or a typed slice (e.g. []*Person).
// If slice is nil, nil is returned. If slice has len zero, an empty slice
// is returned. The function panics if slice is not a slice, or if any element
// of slice is not a struct (excepting nil elements).
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: StructFieldValue, SliceFieldKeyValues.
func SliceFieldValues(fieldName string, slice any) []any {
	if slice == nil {
		return nil
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	iSlice := AnySlice(slice)
	retVals := make([]any, len(iSlice))

	for i := range iSlice {
		retVals[i] = StructFieldValue(fieldName, iSlice[i])
	}

	return retVals
}

// SliceFieldKeyValues is similar to SliceFieldValues, but instead of
// returning a slice of field values, it returns a map containing two
// field values, a "key" and a "value". For example:
//
//	persons := []*person{
//	  {Name: "Alice", Age: 42},
//	  {Name: "Bob", Age: 27},
//	}
//
//	m := SliceFieldKeyValues("Name", "Age", persons)
//	// map[Alice:42 Bob:27]
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// See also: StructFieldValue, SliceFieldValues.
func SliceFieldKeyValues(keyFieldName, valFieldName string, slice any) map[any]any {
	if slice == nil {
		return nil
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	iSlice := AnySlice(slice)
	m := make(map[any]any, len(iSlice))

	for i := range iSlice {
		key := StructFieldValue(keyFieldName, iSlice[i])
		val := StructFieldValue(valFieldName, iSlice[i])

		m[key] = val
	}

	return m
}

// AnySlice converts a typed slice (such as []string) to []any.
// If slice is already of type []any, it is returned unmodified.
// Otherwise a new []any is constructed. If slice is nil, nil is
// returned. The function panics if slice is not a slice.
//
// Note that this function uses reflection, and may panic. It is only
// to be used by test code.
//
// REVISIT: This function predates generics. It can probably be
// removed, or at a minimum, moved to pkg loz.
func AnySlice(slice any) []any {
	if slice == nil {
		return nil
	}

	// If it's already an []interface{}, then just return
	if iSlice, ok := slice.([]any); ok {
		return iSlice
	}

	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		panic(fmt.Sprintf("arg slice expected to be a slice, but was {%T}", slice))
	}

	// Keep the distinction between nil and empty slice input
	if s.IsNil() {
		return nil
	}

	ret := make([]any, s.Len())

	for i := 0; i < s.Len(); i++ {
		ret[i] = s.Index(i).Interface()
	}

	return ret
}

// Name is a convenience function for building a test name to
// pass to t.Run.
//
//	t.Run(testh.Name("my_test", 1), func(t *testing.T) {
//
// The most common usage is with test names that are file
// paths.
//
//	testh.Name("path/to/file") --> "path_to_file"
//
// Any element of arg that prints to empty string is skipped.
func Name(args ...any) string {
	var parts []string
	var s string
	for _, a := range args {
		v := stringz.Val(a)
		s = fmt.Sprintf("%v", v)
		if s == "" {
			continue
		}

		s = strings.ReplaceAll(s, "/", "_")
		s = strings.ReplaceAll(s, ":", "_")
		s = strings.ReplaceAll(s, `\`, "_")
		s = stringz.SanitizeFilename(s)
		s = stringz.EllipsifyASCII(s, 28) // we don't want it to be too long
		parts = append(parts, s)
	}

	s = strings.Join(parts, "_")
	if s == "" {
		return "empty"
	}

	return s
}

// SkipShort invokes t.Skip if testing.Short and arg skip are both true.
func SkipShort(t testing.TB, skip bool) {
	if skip && testing.Short() {
		t.Skip("Skipping long-running test because -short is true.")
	}
}

// AssertCompareFunc matches several of the testify/require funcs.
// It can be used to choose assertion comparison funcs in test cases.
type AssertCompareFunc func(require.TestingT, any, any, ...any)

// Verify that a sample of the require funcs match AssertCompareFunc.
var (
	_ AssertCompareFunc = require.Equal
	_ AssertCompareFunc = require.GreaterOrEqual
	_ AssertCompareFunc = require.Greater
)

// DirCopy copies the contents of sourceDir to a temp dir.
// If keep is false, temp dir will be cleaned up on test exit.
func DirCopy(t testing.TB, sourceDir string, keep bool) (tmpDir string) {
	var err error
	if keep {
		tmpDir, err = os.MkdirTemp("", sanitizeTestName(t.Name())+"_*")
		require.NoError(t, err)
	} else {
		tmpDir = t.TempDir()
	}

	err = copy.Copy(sourceDir, tmpDir)
	require.NoError(t, err)
	t.Logf("Copied %s -> %s", sourceDir, tmpDir)
	return tmpDir
}

// sanitizeTestName sanitizes a test name. This impl is copied
// from testing.T.TempDir.
func sanitizeTestName(name string) string {
	// Drop unusual characters (such as path separators or
	// characters interacting with globs) from the directory name to
	// avoid surprising os.MkdirTemp behavior.
	mapper := func(r rune) rune {
		if r < utf8.RuneSelf {
			const allowed = "!#$%&()+,-.=@^_{}~ "
			if '0' <= r && r <= '9' ||
				'a' <= r && r <= 'z' ||
				'A' <= r && r <= 'Z' {
				return r
			}
			if strings.ContainsRune(allowed, r) {
				return r
			}
		} else if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return r
		}
		return -1
	}
	pattern := strings.Map(mapper, name)
	return pattern
}

// Writer returns an io.Writer whose Write method invokes t.Log.
// A newline is prepended to the log output.
func Writer(t testing.TB) io.Writer {
	return &tWriter{t}
}

var _ io.Writer = (*tWriter)(nil)

type tWriter struct {
	t testing.TB
}

func (t *tWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	t.t.Helper()
	t.t.Log("\n" + string(p))
	return len(p), nil
}

// Chdir changes the working directory to dir, or if dir is empty,
// to a temp dir. On test conclusion, the original working dir is restored,
// and the temp dir deleted (if applicable). The absolute path
// of the changed working dir is returned.
func Chdir(t testing.TB, dir string) (absDir string) {
	origDir, err := os.Getwd()
	require.NoError(t, err)

	if filepath.IsAbs(dir) {
		absDir = dir
	} else {
		absDir, err = filepath.Abs(dir)
		require.NoError(t, err)
	}

	if dir == "" {
		tmpDir := t.TempDir()
		t.Cleanup(func() {
			_ = os.Remove(tmpDir)
		})
		dir = tmpDir
	}

	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	return absDir
}

// SkipWindows skips t if running on Windows.
func SkipWindows(t testing.TB, format string, args ...any) {
	if runtime.GOOS == "windows" {
		t.Skipf(format, args...)
	}
}

// SkipWindowsIf skips t if running on Windows and b is true.
func SkipWindowsIf(t testing.TB, b bool, format string, args ...any) {
	if runtime.GOOS == "windows" && b {
		t.Skipf(format, args...)
	}
}

// WriteTemp writes b to a temporary file. The pattern arg
// is used to generate the file name, per os.CreateTemp.
// If cleanup is true, the file is deleted on test cleanup.
func WriteTemp(t testing.TB, pattern string, b []byte, cleanup bool) (fpath string) {
	f, err := os.CreateTemp("", pattern)
	require.NoError(t, err)

	written, err := f.Write(b)
	require.NoError(t, err)
	fpath = f.Name()
	require.NoError(t, f.Close())

	t.Logf("Wrote %d bytes to: %s", written, fpath)

	if cleanup {
		t.Cleanup(func() {
			assert.NoError(t, os.Remove(fpath))
		})
	}
	return fpath
}

// MustAbsFilepath invokes filepath.Join on elems, and then filepath.Abs
// on the result. It panics on error.
func MustAbsFilepath(elems ...string) string {
	fp := filepath.Join(elems...)
	s, err := filepath.Abs(fp)
	if err != nil {
		panic(err)
	}
	return s
}

// TempDir is the standard means for obtaining a temp dir for tests.
// If arg clean is true, the temp dir is created via t.TempDir, and
// thus is deleted on test cleanup.
func TempDir(t testing.TB, clean bool) string {
	if clean {
		return filepath.Join(t.TempDir(), "sq-test", "tmp")
	}

	fp := filepath.Join(os.TempDir(), "sq-test", stringz.Uniq8(), "tmp")
	require.NoError(t, ioz.RequireDir(fp))
	return fp
}

// TempFile returns the path to a temp file with the given name, in a unique
// temp dir. The file is not created. If arg clean is true, the parent temp
// dir is created via t.TempDir, and thus is deleted on test cleanup.
func TempFile(t testing.TB, name string, clean bool) string {
	fp := filepath.Join(TempDir(t, clean), name)
	return fp
}

// CacheDir is the standard means for obtaining a cache dir for tests.
// If arg clean is true, the cache dir is created via t.TempDir, and
// thus is deleted on test cleanup.
func CacheDir(t testing.TB, clean bool) string {
	if clean {
		return filepath.Join(t.TempDir(), "sq-test", "cache")
	}

	fp := filepath.Join(os.TempDir(), "sq-test", stringz.Uniq8(), "cache")
	require.NoError(t, ioz.RequireDir(fp))
	return fp
}

// ReadFileToString invokes ioz.ReadFileToString, failing t if
// an error occurs.
func ReadFileToString(t testing.TB, name string) string {
	s, err := ioz.ReadFileToString(name)
	require.NoError(t, err)
	return s
}

// ReadToString reads all bytes from r and returns them as a string.
// If r is an io.Closer, it is closed.
func ReadToString(t testing.TB, r io.Reader) string {
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	if r, ok := r.(io.Closer); ok {
		require.NoError(t, r.Close())
	}
	return string(b)
}

// OpenFileCount is a debugging function that returns the count
// of open file handles for the current process via shelling out
// to lsof. This function is skipped on Windows.
// If log is true, the output of lsof is logged.
func OpenFileCount(t testing.TB, log bool) int {
	count, out := doOpenFileCount(t)
	msg := fmt.Sprintf("NewReader files for [%d]: %d", os.Getpid(), count)
	if log {
		msg += "\n\n" + out
	}
	t.Log(msg)
	return count
}

func doOpenFileCount(t testing.TB) (count int, out string) {
	SkipWindows(t, "OpenFileCount not implemented on Windows")

	c := fmt.Sprintf("lsof -p %v", os.Getpid())
	b, err := exec.Command("/bin/sh", "-c", c).Output()
	require.NoError(t, err)
	lines := strings.Split(string(b), "\n")
	count = len(lines) - 1
	return count, string(b)
}

// DiffOpenFileCount is a debugging function that compares the
// open file count at the start of the test with the count at
// the end of the test (via t.Cleanup). This function is skipped on Windows.
func DiffOpenFileCount(t testing.TB, log bool) {
	openingCount, openingOut := doOpenFileCount(t)
	if log {
		t.Logf("START: NewReader files for [%d]: %d\n\n%s", os.Getpid(), openingCount, openingOut)
	}
	t.Cleanup(func() {
		closingCount, closingOut := doOpenFileCount(t)
		if log {
			t.Logf("END: NewReader files for [%d]: %d\n\n%s", os.Getpid(), closingCount, closingOut)
		}
		if openingCount != closingCount {
			t.Logf("NewReader file count changed from %d to %d", openingCount, closingCount)
		} else {
			t.Logf("NewReader file count unchanged: %d", openingCount)
		}
	})
}

// UseProxy sets HTTP_PROXY and HTTPS_PROXY to localhost:9001.
func UseProxy(t testing.TB) {
	t.Setenv("HTTP_PROXY", "http://localhost:9001")
	t.Setenv("HTTPS_PROXY", "http://localhost:9001")
}
