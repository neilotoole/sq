package tu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

// TestSanitizeCwdSegment verifies that a cwd is reduced to a path segment safe
// to nest under a temp dir on any OS. The Windows-drive case is the gh #797
// regression: an absolute cwd outside the project tree (as t.Chdir into a temp
// dir produces) must not inject a drive component like "...\sq\test\C:".
func TestSanitizeCwdSegment(t *testing.T) {
	const unixProj = "/home/user/work/sq/sq"
	const winProj = `C:\work\sq\sq`
	testCases := []struct {
		name    string
		dir     string
		projDir string
		want    string
	}{
		{name: "inside_project_unix", dir: unixProj + "/drivers/sqlite3", projDir: unixProj, want: "drivers/sqlite3"},
		{name: "project_root_unix", dir: unixProj, projDir: unixProj, want: ""},
		{
			name: "abs_outside_project_unix", dir: "/var/folders/xy/tmp.dollar",
			projDir: unixProj, want: "var/folders/xy/tmp.dollar",
		},
		{name: "inside_project_win", dir: winProj + `\drivers\sqlite3`, projDir: winProj, want: `drivers\sqlite3`},
		{
			name: "drive_outside_project_win", dir: `C:\Users\RUNNER~1\Temp\dollar$dir`,
			projDir: winProj, want: `Users\RUNNER~1\Temp\dollar$dir`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeCwdSegment(tc.dir, tc.projDir)
			require.Equal(t, tc.want, got)
			require.NotContains(t, got, ":", "result must not contain a volume colon")
			require.False(t, strings.HasPrefix(got, "/") || strings.HasPrefix(got, `\`),
				"result must not start with a path separator")
		})
	}
}

// TestFieldExtractionFunctions tests StructFieldValue, SliceFieldValues,
// SliceFieldKeyValues.
func TestFieldExtractionFunctions(t *testing.T) {
	type person struct {
		UUID     string
		Age      int
		Nickname *string
	}

	p1 := &person{
		UUID:     "235a50d7-3955-431c-8641-6ce171abf589",
		Age:      42,
		Nickname: nil,
	}

	nn := "The Great"
	p2 := &person{
		UUID:     "81975a8f-6add-441a-8c81-3806a9f4c6f0",
		Age:      27,
		Nickname: &nn,
	}

	uu := StructFieldValue("UUID", p1)
	require.Equal(t, uu, p1.UUID)
	age := StructFieldValue("Age", p1)
	require.Equal(t, age, 42)

	require.Panics(t, func() {
		_ = StructFieldValue("UUID", 123)
	}, "non-struct arg should panic")

	require.Nil(t, StructFieldValue("UUID", nil))

	require.Panics(t, func() {
		_ = StructFieldValue("", p1)
	}, "invalid fieldName should panic")

	require.Panics(t, func() {
		_ = StructFieldValue("NotAField", p1)
	}, "invalid fieldName should panic")

	nickname := StructFieldValue("Nickname", p1)
	require.Nil(t, nickname)

	nickname = StructFieldValue("Nickname", p2)
	require.NotNil(t, nickname)
	require.EqualValues(t, nickname, p2.Nickname)

	iSlice := []any{p1, p2}
	iVals := SliceFieldValues("UUID", iSlice)
	require.Len(t, iVals, 2)
	require.Equal(t, p1.UUID, iVals[0])

	personPtrSlice := []*person{p1, p2}
	iVals2 := SliceFieldValues("UUID", personPtrSlice)
	require.Len(t, iVals2, 2)
	require.EqualValues(t, iVals, iVals2)

	personSlice := []person{*p1, *p2}
	iVals3 := SliceFieldValues("UUID", personSlice)
	require.Len(t, iVals2, 2)
	require.EqualValues(t, iVals, iVals3)

	require.Panics(t, func() {
		_ = SliceFieldValues("UUID", p1)
	}, "non-slice arg should panic")

	m1 := SliceFieldKeyValues("UUID", "Age", iSlice)
	require.Len(t, m1, 2)

	require.Equal(t, m1[p1.UUID], p1.Age)
	require.Equal(t, m1[p2.UUID], p2.Age)
}

func TestInterfaceSlice(t *testing.T) {
	stringSlice := []string{"hello", "world"}
	iSlice := AnySlice(stringSlice)
	require.Equal(t, len(stringSlice), len(iSlice))
	require.Equal(t, stringSlice[0], iSlice[0])

	iSlice = AnySlice(nil)
	require.Nil(t, iSlice)

	require.Panics(t, func() {
		_ = AnySlice(42)
	}, "should panic for non-slice arg")
}

func TestTempDir(t *testing.T) {
	log := lgt.New(t)
	log.Debug("huzzxah")

	td1 := TempDir(t)
	t.Logf("td1: %s", td1)
	require.NotEmpty(t, td1)
	require.DirExists(t, td1)

	td2 := TempDir(t)
	t.Logf("td2: %s", td2)

	require.NotEqual(t, td1, td2)

	td3 := TempDir(t, "foo", "bar")
	t.Logf("td3: %s", td3)
	require.True(t, strings.HasSuffix(td3, filepath.Join("foo", "bar")))
}

func TestGenerateBinaryFile(t *testing.T) {
	fp := GenerateBinaryFile(t, ".", 1024, false)
	require.FileExists(t, fp)
	gotSize, err := ioz.Filesize(fp)
	require.NoError(t, err)
	require.Equal(t, int64(1024), gotSize)
	require.NoError(t, os.Remove(fp))

	fp = GenerateBinaryFile(t, ".", 1024*1024, true)
	require.FileExists(t, fp)
	gotSize, err = ioz.Filesize(fp)
	require.NoError(t, err)
	require.Equal(t, int64(1024*1024), gotSize)
}

// fakeTB is a testing.TB for exercising TempDir's cleanup. It records
// cleanups instead of registering them, reports a settable Failed, and
// records Logf and Errorf calls. Everything else goes to the embedded
// testing.TB.
type fakeTB struct {
	testing.TB

	mu       sync.Mutex
	failed   bool
	cleanups []func()
	logs     []string
	errs     []string
}

func newFakeTB(t *testing.T) *fakeTB {
	t.Helper()
	return &fakeTB{TB: t}
}

func (f *fakeTB) Cleanup(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanups = append(f.cleanups, fn)
}

func (f *fakeTB) Failed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failed
}

func (f *fakeTB) Logf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

func (f *fakeTB) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	f.TB.Logf("fakeTB.Errorf: %s", msg)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, msg)
}

// runCleanups runs the recorded cleanups in reverse order, as testing does.
// Also like testing, it runs cleanups that are registered while it runs.
func (f *fakeTB) runCleanups() {
	for {
		f.mu.Lock()
		if len(f.cleanups) == 0 {
			f.mu.Unlock()
			return
		}
		last := len(f.cleanups) - 1
		fn := f.cleanups[last]
		f.cleanups = f.cleanups[:last]
		f.mu.Unlock()

		fn()
	}
}

// TestTempDir_RemovedOnPass verifies that a passing test's temp dirs, and
// their empty <pid> parent dir, are removed by the cleanup.
func TestTempDir_RemovedOnPass(t *testing.T) {
	fake := newFakeTB(t)
	d1 := TempDir(fake)
	d2 := TempDir(fake)
	d3 := TempDir(fake, "foo", "bar")
	require.NoError(t, os.WriteFile(filepath.Join(d3, "data.txt"), []byte("hello"), 0o600))
	pidDir := filepath.Dir(d1)

	fake.runCleanups()

	require.Empty(t, fake.errs)
	require.NoDirExists(t, d1)
	require.NoDirExists(t, d2)
	require.NoDirExists(t, filepath.Dir(filepath.Dir(d3)))
	require.NoDirExists(t, pidDir)

	tempDirsMu.Lock()
	_, ok := tempDirs[fake]
	tempDirsMu.Unlock()
	require.False(t, ok, "tempDirs entry should be deleted by the cleanup")
}

// TestTempDir_KeptOnFail verifies that a failed test's temp dirs are kept,
// and that the <pid> parent dir is logged.
func TestTempDir_KeptOnFail(t *testing.T) {
	fake := newFakeTB(t)
	d1 := TempDir(fake)
	pidDir := filepath.Dir(d1)
	t.Cleanup(func() { _ = os.RemoveAll(pidDir) })

	fake.failed = true
	fake.runCleanups()

	require.DirExists(t, d1)
	require.Empty(t, fake.errs)
	require.Len(t, fake.logs, 1)
	require.Equal(t, "temp dirs kept for failed test: "+pidDir, fake.logs[0])
}

// TestTempDir_OneCleanupPerTB verifies that RegisterTempDirCleanup and
// TempDir together register exactly one cleanup per test.
func TestTempDir_OneCleanupPerTB(t *testing.T) {
	fake := newFakeTB(t)
	RegisterTempDirCleanup(fake)
	RegisterTempDirCleanup(fake)
	for range 3 {
		TempDir(fake)
	}

	require.Len(t, fake.cleanups, 1)
	fake.runCleanups()
	require.Empty(t, fake.errs)
}

// TestRegisterTempDirCleanup_Order verifies that a cleanup registered after
// RegisterTempDirCleanup, but before the dir is created, runs while the dir
// still exists. This is how testh.New keeps fixture copies until
// Helper.Close has closed them.
func TestRegisterTempDirCleanup_Order(t *testing.T) {
	fake := newFakeTB(t)
	RegisterTempDirCleanup(fake)

	var dir string
	var existedAtClose bool
	fake.Cleanup(func() { existedAtClose = ioz.DirExists(dir) })

	dir = TempDir(fake)
	fake.runCleanups()

	require.True(t, existedAtClose, "dir removed before the later-registered cleanup ran")
	require.NoDirExists(t, dir)
}

// TestTempDir_AfterCleanupRan verifies that a TempDir call made by a cleanup
// that runs after tb's removal cleanup gets a new removal cleanup.
func TestTempDir_AfterCleanupRan(t *testing.T) {
	fake := newFakeTB(t)

	var late string
	fake.Cleanup(func() { late = TempDir(fake) }) // Registered first, so runs last.
	early := TempDir(fake)

	fake.runCleanups()

	require.Empty(t, fake.errs)
	require.NoDirExists(t, early)
	require.NotEmpty(t, late)
	require.NoDirExists(t, late)
}

// TestTempDir_RemoveError verifies that a removal failure fails the test,
// and that the <pid> parent dir is then left alone.
func TestTempDir_RemoveError(t *testing.T) {
	if isWindows {
		t.Skip("Windows dir permissions don't block removal this way")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores dir permissions")
	}

	fake := newFakeTB(t)
	d := TempDir(fake, "locked")
	require.NoError(t, os.WriteFile(filepath.Join(d, "f.txt"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(d, 0o500))
	pidDir := filepath.Dir(filepath.Dir(d))
	t.Cleanup(func() {
		_ = os.Chmod(d, 0o700)
		_ = os.RemoveAll(pidDir)
	})

	fake.runCleanups()

	require.Len(t, fake.errs, 1)
	require.Contains(t, fake.errs[0], "remove temp dir")
	require.DirExists(t, pidDir)
}
