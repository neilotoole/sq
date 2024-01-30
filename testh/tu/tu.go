// Package tu contains basic generic test utilities.
package tu

import (
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/neilotoole/sq/testh/proj"

	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

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

// AssertCompareFunc matches several of the testify/require funcs.
// It can be used to choose assertion comparison funcs in test cases.
type AssertCompareFunc func(require.TestingT, any, any, ...any)

// Verify that a sample of the require funcs match AssertCompareFunc.
var (
	_ AssertCompareFunc = require.Equal
	_ AssertCompareFunc = require.GreaterOrEqual
	_ AssertCompareFunc = require.Greater
)

// RequireNoTake fails if a value is taken from c.
func RequireNoTake[C any](tb testing.TB, c <-chan C, msgAndArgs ...any) {
	tb.Helper()
	select {
	case <-c:
		require.Fail(tb, "unexpected take from channel", msgAndArgs...)
	default:
	}
}

// RequireTake fails if a value is not taken from c.
func RequireTake[C any](tb testing.TB, c <-chan C, msgAndArgs ...any) {
	tb.Helper()
	select {
	case <-c:
	default:
		require.Fail(tb, "unexpected failure to take from channel", msgAndArgs...)
	}
}

// DirCopy copies the contents of sourceDir to a temp dir.
func DirCopy(tb testing.TB, sourceDir string) (tmpDir string) {
	tb.Helper()

	tmpDir = TempDir(tb)
	err := copy.Copy(sourceDir, tmpDir)
	require.NoError(tb, err)
	tb.Logf("Copied %s -> %s", sourceDir, tmpDir)
	return tmpDir
}

// sanitizeTestName sanitizes a test name. This impl is copied
// from testing.T.TempDir.
func sanitizeTestName(name string) string { //nolint:unused
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

// Writer returns an io.Writer whose Write method invokes tb.Log.
// A newline is prepended to the log output.
func Writer(tb testing.TB) io.Writer { //nolint:thelper
	return &tWriter{tb}
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
func Chdir(tb testing.TB, dir string) (absDir string) {
	tb.Helper()
	origDir, err := os.Getwd()
	require.NoError(tb, err)

	if filepath.IsAbs(dir) {
		absDir = dir
	} else {
		absDir, err = filepath.Abs(dir)
		require.NoError(tb, err)
	}

	if dir == "" {
		tmpDir := TempDir(tb)
		tb.Cleanup(func() {
			_ = os.Remove(tmpDir)
		})
		dir = tmpDir
	}

	require.NoError(tb, os.Chdir(dir))
	tb.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	return absDir
}

// WriteTemp writes b to a temporary file. The pattern arg
// is used to generate the file name, per os.CreateTemp.
// If cleanup is true, the file is deleted on test cleanup.
func WriteTemp(tb testing.TB, pattern string, b []byte, cleanup bool) (fpath string) {
	tb.Helper()
	f, err := os.CreateTemp("", pattern)
	require.NoError(tb, err)

	written, err := f.Write(b)
	require.NoError(tb, err)
	fpath = f.Name()
	require.NoError(tb, f.Close())

	tb.Logf("Wrote %d bytes to: %s", written, fpath)

	if cleanup {
		tb.Cleanup(func() {
			assert.NoError(tb, os.Remove(fpath))
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

// MustStat invokes os.Stat on fp, and fails t on error.
func MustStat(tb testing.TB, fp string) os.FileInfo {
	tb.Helper()
	fi, err := os.Stat(fp)
	require.NoError(tb, err)
	return fi
}

func randString() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	cs := crc32.ChecksumIEEE(b)
	return fmt.Sprintf("%x", cs)
}

var dirCount = &atomic.Int64{}

// TempDir is the standard means for obtaining a temp dir for tests.
// A new, unique temp dir is returned on each call. If arg subs is
// non-empty, that sub-directory structure is created within the
// parent temp dir. The returned value is an absolute path of
// the form:
//
//	# tu.TempDir(t):
//	/var/folders/68/qthw...0gn/T/sq/test/testh/tu/TestTempDir/69226/2_1706579687990637_f8f226d0
//
//	# tu.TempDir(t, "foo", "bar"):
//	/var/folders/68/qthw...0gn/T/sq/test/testh/tu/TestTempDir/69226/3_1706579687990706_efb99710/foo/bar
//
// The returned dir is a subdir of os.TempDir(), and includes the package path
// and test name (sanitized), as well as the pid, created dir count, timestamp,
// and random value. We use this structure to make it easier to identify the
// calling test when debugging. The dir is created with perms 0777.
//
// The caller is responsible for removing the dir if desired - it is NOT
// automatically deleted via t.Cleanup.
func TempDir(tb testing.TB, subs ...string) string {
	tb.Helper()

	dir, err := os.Getwd()
	require.NoError(tb, err)
	dir = strings.TrimPrefix(dir, proj.Dir())

	fp := filepath.Join(
		os.TempDir(),
		"sq",
		"test",
		dir,
		stringz.SanitizeFilename(tb.Name()),
		strconv.Itoa(os.Getpid()),
		fmt.Sprintf(
			"%d_%d_%s",
			dirCount.Add(1),
			time.Now().UnixMicro(),
			randString(),
		))

	for _, sub := range subs {
		fp = filepath.Join(fp, sub)
	}

	err = os.MkdirAll(fp, 0o777)
	require.NoError(tb, err)
	return fp
}

// TempFile returns the path to a temp file with the given name, in a unique
// temp dir. The file is not created.
func TempFile(tb testing.TB, name string) string {
	tb.Helper()
	fp := filepath.Join(TempDir(tb), name)
	return fp
}

// ReadFileToString invokes ioz.ReadFileToString, failing t if
// an error occurs.
func ReadFileToString(tb testing.TB, name string) string {
	tb.Helper()
	s, err := ioz.ReadFileToString(name)
	require.NoError(tb, err)
	return s
}

// ReadToString reads all bytes from r and returns them as a string.
// If r is an io.Closer, it is closed.
func ReadToString(tb testing.TB, r io.Reader) string {
	tb.Helper()
	b, err := io.ReadAll(r)
	require.NoError(tb, err)
	if r, ok := r.(io.Closer); ok {
		require.NoError(tb, r.Close())
	}
	return string(b)
}

// OpenFileCount is a debugging function that returns the count
// of open file handles for the current process via shelling out
// to lsof. On the master branch, OpenFileCount probably should never
// be called; it's a debugging function for use during development.
//
// If arg log is true, the output of lsof is logged.
//
// This function is skipped on Windows.
func OpenFileCount(tb testing.TB, log bool, label string) int {
	tb.Helper()
	count, out := doOpenFileCount(tb)
	var msg string

	if label != "" {
		msg = "\n\n" + strings.Repeat("=", 40) + "\n" +
			label + "\n" + strings.Repeat("=", 40) + "\n\n"
	}

	msg += fmt.Sprintf("Open files for pid [%d]: %d", os.Getpid(), count)
	if log {
		msg += "\n\n" + out
	}
	tb.Log(msg)
	return count
}

func doOpenFileCount(tb testing.TB) (count int, out string) {
	tb.Helper()
	SkipWindows(tb, "OpenFileCount not implemented on Windows")

	c := fmt.Sprintf("lsof -p %v", os.Getpid())
	b, err := exec.Command("/bin/sh", "-c", c).Output()
	require.NoError(tb, err)
	lines := strings.Split(string(b), "\n")
	count = len(lines) - 1
	return count, string(b)
}

// DiffOpenFileCount is a debugging function that compares the
// open file count at the start of the test with the count at
// the end of the test (via t.Cleanup). This function is skipped on Windows.
func DiffOpenFileCount(tb testing.TB, log bool) {
	tb.Helper()
	openingCount, openingOut := doOpenFileCount(tb)
	if log {
		tb.Logf("START: Open files for pid [%d]: %d\n\n%s", os.Getpid(), openingCount, openingOut)
	}
	tb.Cleanup(func() {
		closingCount, closingOut := doOpenFileCount(tb)
		if log {
			tb.Logf("END: Open files for pid [%d]: %d\n\n%s", os.Getpid(), closingCount, closingOut)
		}
		if openingCount != closingCount {
			tb.Logf("Open file count changed from %d to %d", openingCount, closingCount)
		} else {
			tb.Logf("Open file count unchanged: %d", openingCount)
		}
	})
}

// UseProxy sets HTTP_PROXY and HTTPS_PROXY to localhost:9001.
func UseProxy(tb testing.TB) {
	tb.Helper()
	tb.Setenv("HTTP_PROXY", "http://localhost:9001")
	tb.Setenv("HTTPS_PROXY", "http://localhost:9001")
}
