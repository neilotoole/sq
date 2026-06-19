package files_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

func newTestFiles(t *testing.T) (context.Context, *files.Files) {
	t.Helper()
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	optReg := &options.Registry{}
	optReg.Add(files.OptCacheLockTimeout)
	fs, err := files.New(
		ctx,
		optReg,
		testh.TempLockFunc(t),
		tu.TempDir(t, "temp"),
		tu.TempDir(t, "cache"),
	)
	require.NoError(t, err)
	return ctx, fs
}

// populateLeaf creates a fake ingest-cache leaf dir with a cache DB file,
// returning the cache DB path.
func populateLeaf(t *testing.T, leafDir string) (cacheDB string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(leafDir, 0o700))
	cacheDB = filepath.Join(leafDir, "cache.sqlite.db")
	require.NoError(t, os.WriteFile(cacheDB, []byte("fake"), 0o600))
	return cacheDB
}

// TestFiles_CacheClearSourceAll_ClearsAllLeaves verifies that every cache
// leaf under the handle's dir is cleared, not just the leaf for the
// source's current location hash: leaves accumulate when the location
// (e.g. a rotated secret), options, or schema change.
func TestFiles_CacheClearSourceAll_ClearsAllLeaves(t *testing.T) {
	ctx, fs := newTestFiles(t)

	src := &source.Source{Handle: "@prod", Type: drivertype.CSV, Location: "/tmp/a.csv"}

	currentLeaf, currentDB, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	populateLeaf(t, currentLeaf)

	// A leaf left over from a previous location hash.
	staleDB := populateLeaf(t, filepath.Join(filepath.Dir(currentLeaf), "deadbeef"))

	require.NoError(t, fs.CacheClearSourceAll(ctx, src, []string{src.Handle}))
	require.False(t, ioz.FileAccessible(currentDB))
	require.False(t, ioz.FileAccessible(staleDB))
}

// TestFiles_CacheClearSourceAll_PreservesNestedHandleDirs verifies that
// clearing @prod doesn't touch cache dirs of a source whose handle nests
// under it (e.g. @prod/db/x): such nesting can exist in legacy or
// hand-edited configs, and the nested source's cache dirs live below
// @prod's cache dir on disk.
func TestFiles_CacheClearSourceAll_PreservesNestedHandleDirs(t *testing.T) {
	ctx, fs := newTestFiles(t)

	parent := &source.Source{Handle: "@prod", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	nested := &source.Source{Handle: "@prod/db/x", Type: drivertype.CSV, Location: "/tmp/b.csv"}

	parentLeaf, parentDB, _, err := fs.CachePaths(parent)
	require.NoError(t, err)
	populateLeaf(t, parentLeaf)

	nestedLeaf, nestedDB, _, err := fs.CachePaths(nested)
	require.NoError(t, err)
	populateLeaf(t, nestedLeaf)

	require.NoError(t, fs.CacheClearSourceAll(ctx, parent,
		[]string{parent.Handle, nested.Handle}))
	require.False(t, ioz.FileAccessible(parentDB))
	require.True(t, ioz.FileAccessible(nestedDB),
		"clearing @prod must not clear @prod/db/x's cache")
}

// TestFiles_CacheClearSourceAll_BlockedByLockHolder verifies that clear
// respects the per-leaf cache lock: a concurrent ingest (Grips.OpenIngest)
// holds the leaf's pid.lock for the duration of the ingest, and clear must
// not yank the cache dir out from under it.
func TestFiles_CacheClearSourceAll_BlockedByLockHolder(t *testing.T) {
	ctx, fs := newTestFiles(t)

	src := &source.Source{Handle: "@prod", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	leafDir, cacheDB, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	populateLeaf(t, leafDir)

	// Simulate another process holding the leaf's cache lock. The pid
	// must belong to a live foreign process: the lockfile package treats
	// a lock held by our own pid as acquirable, so the test process's
	// parent pid is used.
	lockPath := filepath.Join(leafDir, "pid.lock")
	require.NoError(t, os.WriteFile(lockPath,
		[]byte(strconv.Itoa(os.Getppid())+"\n"), 0o600))

	// Short lock timeout so the test fails fast.
	ctx = options.NewContext(ctx, options.Options{
		files.OptCacheLockTimeout.Key(): "25ms",
	})

	err = fs.CacheClearSourceAll(ctx, src, []string{src.Handle})
	require.Error(t, err,
		"clear must not proceed while another process holds the leaf's cache lock")
	require.True(t, ioz.FileAccessible(cacheDB),
		"the locked leaf's cache must be untouched")
}
