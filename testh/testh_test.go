package testh_test

import (
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/ryboe/q" // keep the q lib around
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/drivers/duckdb"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestVal(t *testing.T) {
	want := "hello"
	var got any

	if stringz.Val(nil) != nil {
		t.FailNow()
	}

	var v0 any
	if stringz.Val(v0) != nil {
		t.FailNow()
	}

	v1 := want
	var v1a any = want
	v2 := &v1
	var v3 any = &v1
	v4 := &v2
	v5 := &v4

	vals := []any{v1, v1a, v2, v3, v4, v5}
	for _, val := range vals {
		got = stringz.Val(val)

		if got != want {
			t.Errorf("expected %T(%v) but got %T(%v)", want, want, got, got)
		}
	}

	slice := []string{"a", "b"}
	require.Equal(t, slice, stringz.Val(slice))
	require.Equal(t, slice, stringz.Val(&slice))

	b := true
	require.Equal(t, b, stringz.Val(b))
	require.Equal(t, b, stringz.Val(&b))

	type structT struct {
		f string
	}

	st1 := structT{f: "hello"}
	require.Equal(t, st1, stringz.Val(st1))
	require.Equal(t, st1, stringz.Val(&st1))

	var c chan int
	require.Nil(t, stringz.Val(c))
	c = make(chan int, 10)
	require.Equal(t, c, stringz.Val(c))
	require.Equal(t, c, stringz.Val(&c))
}

func TestCopyRecords(t *testing.T) {
	v1, v2, v3, v4, v5, v6 := int64(1), float64(1.1), true, "hello", []byte("hello"), time.Unix(0, 0)

	testCases := map[string][]record.Record{
		"nil":   nil,
		"empty": {},
		"vals": {
			{nil, v1, v2, v3, v4, v5, v6},
			// {nil, &v1, &v2, &v3, &v4, &v5, &v6},
		},
	}

	for name, recs := range testCases {
		t.Run(name, func(t *testing.T) {
			recs2 := record.CloneSlice(recs)
			require.True(t, len(recs) == len(recs2))

			if recs == nil {
				require.True(t, recs2 == nil)
				return
			}

			for i := range recs {
				require.True(t, len(recs[i]) == len(recs2[i]))
				for j := range recs[i] {
					if recs[i][j] == nil {
						require.True(t, recs2[i][j] == nil)
						continue
					}

					val1, val2 := stringz.Val(recs[i][j]), stringz.Val(recs2[i][j])
					require.Equal(t, val1, val2,
						"dereferenced values should be equal: %#v --> %#v", val1, val2)
				}
			}
		})
	}
}

func TestRecordsFromTbl(t *testing.T) {
	recMeta1, recs1 := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
	require.Equal(t, sakila.TblActorColKinds(), recMeta1.Kinds())

	recs1[0][0] = t.Name()

	recMeta2, recs2 := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
	require.False(t, &recMeta1 == &recMeta2, "should be distinct copies")
	require.False(t, &recs1 == &recs2, "should be distinct copies")
	require.NotEqual(t, recs1[0][0], recs2[0][0], "recs2 should not have the mutated value from recs1")
}

func TestHelper_Files(t *testing.T) {
	fpath := "drivers/csv/testdata/person.csv"
	wantBytes := proj.ReadFile(fpath)

	src := &source.Source{
		Handle:   "@test_" + stringz.Uniq8(),
		Type:     drivertype.CSV,
		Location: proj.Abs(fpath),
	}

	th := testh.New(t)
	fs := th.Files()

	typ, err := fs.DetectType(th.Context, src.Handle, src.Location)
	require.NoError(t, err)
	require.Equal(t, src.Type, typ)

	g, _ := errgroup.WithContext(th.Context)

	for range 1000 {
		g.Go(func() error {
			r, fErr := fs.NewReader(th.Context, src, false)
			require.NoError(t, fErr)

			defer func() { require.NoError(t, r.Close()) }()

			b, fErr := io.ReadAll(r)
			require.NoError(t, fErr)

			require.Equal(t, wantBytes, b)
			return nil
		})
	}

	err = g.Wait()
	require.NoError(t, err)
}

func TestTName(t *testing.T) {
	testCases := []struct {
		a    []any
		want string
	}{
		{a: []any{}, want: "empty"},
		{a: []any{"test", 1}, want: "test_1"},
		{a: []any{"/file/path/name"}, want: "_file_path_name"},
	}

	for _, tc := range testCases {
		got := tu.Name(tc.a...)
		require.Equal(t, tc.want, got)
	}
}

// cleanupTB is a testing.TB that records cleanups instead of registering
// them, so that a test can run them one at a time.
type cleanupTB struct {
	testing.TB

	mu       sync.Mutex
	cleanups []func()
}

func (c *cleanupTB) Cleanup(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanups = append(c.cleanups, fn)
}

// runCleanup runs c's cleanup at index i, unless it has already run.
func (c *cleanupTB) runCleanup(i int) {
	c.mu.Lock()
	fn := c.cleanups[i]
	c.cleanups[i] = nil
	c.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// runRemainingCleanups runs any of c's recorded cleanups that haven't already
// run, in reverse registration order, as testing does.
func (c *cleanupTB) runRemainingCleanups() {
	c.mu.Lock()
	n := len(c.cleanups)
	c.mu.Unlock()
	for i := n - 1; i >= 0; i-- {
		c.runCleanup(i)
	}
}

// TestHelper_TempDirCleanup verifies that a Helper's temp dirs (fixture
// copies, and the Files temp and cache dirs) are removed when the test
// passes, and that removal runs only after Helper.Close, which closes the
// files inside those dirs (gh #1162).
func TestHelper_TempDirCleanup(t *testing.T) {
	t.Run("removed_on_pass", func(t *testing.T) {
		var dirs []string
		// If the inner subtest fails, stop here instead of falling through to
		// the NoDirExists assertions below, which would add a second,
		// misleading failure: the inner subtest's dirs are kept on failure, by
		// design.
		if !t.Run("helper", func(t *testing.T) {
			th := testh.New(t)

			sl3 := th.Source(sakila.SL3)
			th.Open(sl3)
			sl3Path, err := sqlite3.PathFromLocation(sl3)
			require.NoError(t, err)

			duck := th.Source(sakila.Duck)
			th.Open(duck)
			duckPath, err := duckdb.PathFromLocation(duck)
			require.NoError(t, err)

			fs := th.Files()
			dirs = []string{
				filepath.Dir(sl3Path),
				filepath.Dir(duckPath),
				filepath.Dir(fs.TempDir()),
				filepath.Dir(fs.CacheDir()),
			}
			for _, dir := range dirs {
				require.DirExists(t, dir)
			}
		}) {
			t.FailNow()
		}

		require.Len(t, dirs, 4)
		for _, dir := range dirs {
			require.NoDirExists(t, dir)
		}
	})

	t.Run("removed_after_close", func(t *testing.T) {
		ctb := &cleanupTB{TB: t}
		th := testh.New(ctb)

		// If a require below fails before the manual cleanup loop runs, this
		// still runs ctb's recorded cleanups (including Helper.Close), so the
		// Helper doesn't leak its grip, dirs, and tu map entry.
		t.Cleanup(ctb.runRemainingCleanups)

		src := th.Source(sakila.SL3)
		th.Open(src)
		path, err := sqlite3.PathFromLocation(src)
		require.NoError(t, err)

		ctb.mu.Lock()
		n := len(ctb.cleanups)
		ctb.mu.Unlock()
		require.GreaterOrEqual(t, n, 2)

		// Run every cleanup except the first registered, in reverse order as
		// testing does. These include Helper.Close.
		for i := n - 1; i > 0; i-- {
			ctb.runCleanup(i)
		}
		require.FileExists(t, path, "fixture copy removed before Helper.Close ran")

		// The first registered cleanup is the temp dir removal.
		ctb.runCleanup(0)
		require.NoFileExists(t, path)
	})
}

// TestHelper_QuerySLQ_UsesFixtureCopy verifies that a Helper query runs
// against the per-test fixture copy, not the version-controlled original.
// Helper.Source copies the fixture, but the Helper's collection used to keep
// the original location, so libsq opened the original read-write. On Windows
// that exclusive open blocks other packages copying the fixture (gh #1162).
func TestHelper_QuerySLQ_UsesFixtureCopy(t *testing.T) {
	testCases := []struct {
		handle   string
		prefix   string
		origPath string
		pathFn   func(src *source.Source) (string, error)
	}{
		{handle: sakila.SL3, prefix: sqlite3.Prefix, origPath: proj.Abs(sakila.PathSL3), pathFn: sqlite3.PathFromLocation},
		{handle: sakila.Duck, prefix: duckdb.Prefix, origPath: proj.Abs(sakila.PathDuck), pathFn: duckdb.PathFromLocation},
	}

	for _, tc := range testCases {
		t.Run(tc.handle, func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(tc.handle)
			copyPath, err := tc.pathFn(src)
			require.NoError(t, err)
			require.NotEqual(t, tc.origPath, copyPath, "Helper.Source must return a copy")

			_, err = th.QuerySLQ(tc.handle+` | .actor | .[0:1]`, nil)
			require.NoError(t, err)

			// Probe the cache with a source that points at the ORIGINAL
			// fixture, not the copy. The cache is keyed by (mode, handle), so
			// a hit returns the grip the query opened and the probe's
			// location is ignored; a miss would open the original named by
			// the probe, and the assertions below then fail rather than
			// passing vacuously.
			probe := src.Clone()
			probe.Location = tc.prefix + tc.origPath
			grip, err := th.Grips().Open(th.Context, probe, driver.ModeReadWrite)
			require.NoError(t, err)
			gotPath, err := tc.pathFn(grip.Source())
			require.NoError(t, err)

			require.Equal(t, copyPath, gotPath, "query must use the per-test copy")
			require.NotEqual(t, tc.origPath, gotPath, "query must not open the original fixture")
		})
	}
}
