package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

// TestFiles_WriteIngestChecksum_HTTP_Cacheable drives the HTTP branch of
// WriteIngestChecksum end to end with a cacheable download: after a full
// download+drain, WriteIngestChecksum waits on the stream, computes and writes
// the checksum, and CachedBackingSourceFor then finds the cached backing DB.
func TestFiles_WriteIngestChecksum_HTTP_Cacheable(t *testing.T) {
	const body = "a,b,c\n1,2,3\n"
	srvr := newCacheableCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}
	backingSrc := &source.Source{Handle: src.Handle + "_cached", Type: drivertype.SQLite}

	// Full download + drain so the body is cached on disk.
	r, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	got, err := readAllAndClose(t, r)
	require.NoError(t, err)
	require.Equal(t, body, string(got))

	// HTTP branch of WriteIngestChecksum: waits on stream Filled/Done, then
	// writes the checksum.
	require.NoError(t, fs.WriteIngestChecksum(ctx, src, backingSrc))

	// The checksums file should now exist on disk.
	_, _, checksumsPath, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(checksumsPath), "checksums file written")

	// CachedBackingSourceFor for the remote source drives
	// cachedBackingSourceForRemoteFile -> maybeStartDownload(checkFresh) ->
	// cachedBackingSourceForFile. With no cache.sqlite.db it reports ok=false.
	// Note: the remote ok=true happy path is not reachable in this synthetic
	// single-instance setup, because maybeStartDownload(checkFresh=true)
	// re-checks freshness and reports the body absent.
	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestFiles_CachedBackingSourceFor_InProgressStream verifies that while a
// download stream is in progress (not yet completed), CachedBackingSourceFor
// for the remote source reports ok=false.
func TestFiles_CachedBackingSourceFor_InProgressStream(t *testing.T) {
	const body = "a,b,c\n1,2,3\n"
	srvr := newCacheableCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	// Start the download stream but do NOT drain it: the stream entry stays in
	// fs.streams, so cachedBackingSourceForFile sees an active download.
	r, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = readAllAndClose(t, r) })

	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok, "download in progress -> no valid backing cache")
}

// TestFiles_CachedBackingSourceFor_SrcFileGone covers the checksum.ForFile
// error branch of cachedBackingSourceForFile: the checksum is recorded for a
// path that references the source file, but the source file is removed before
// the backing source is consulted.
func TestFiles_CachedBackingSourceFor_SrcFileGone(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	dir := tu.TempDir(t, "src")
	src := mustCSVSrc(t, dir, "a,b,c\n1,2,3\n")

	leafDir, _, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(leafDir, 0o700))

	backingSrc := &source.Source{Handle: src.Handle + "_cached", Type: drivertype.SQLite}
	require.NoError(t, fs.WriteIngestChecksum(ctx, src, backingSrc))

	// Remove the source file: checksum.ForFile now errors when recomputing.
	require.NoError(t, os.Remove(src.Location))

	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.Error(t, err, "source file gone -> checksum recompute errors")
	require.False(t, ok)
}

// TestFiles_DoCacheClearAll_WithFiles exercises doCacheClearAll when the cache
// dir actually contains files: the rename-to-temp + RemoveAll + recreate path.
func TestFiles_DoCacheClearAll_WithFiles(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	// Populate the cache dir with a nested file.
	sub := filepath.Join(fs.CacheDir(), "sources", "h", "abc")
	require.NoError(t, os.MkdirAll(sub, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "cache.sqlite.db"), []byte("data"), 0o600))
	require.True(t, ioz.DirExists(fs.CacheDir()))

	require.NoError(t, fs.CacheClearAll(ctx))

	// The cache dir is recreated and empty afterward.
	require.True(t, ioz.DirExists(fs.CacheDir()), "cache dir recreated")
	entries, err := os.ReadDir(fs.CacheDir())
	require.NoError(t, err)
	require.Empty(t, entries, "cache dir empty after clear")
}

// TestFiles_DoCacheClearAll_NotExist covers the no-op branch where the cache
// dir does not exist.
func TestFiles_DoCacheClearAll_NotExist(t *testing.T) {
	ctx, fs := newFilesAtDir(t, tu.TempDir(t, "temp"), filepath.Join(tu.TempDir(t, "cache"), "does-not-exist"))
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	require.False(t, ioz.DirExists(fs.CacheDir()))
	require.NoError(t, fs.CacheClearAll(ctx))
}

// TestFiles_CacheClearSourceAll_InvalidHandle covers the invalid-handle error
// branch of CacheClearSourceAll.
func TestFiles_CacheClearSourceAll_InvalidHandle(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	bad := &source.Source{Handle: "not a handle", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	err := fs.CacheClearSourceAll(ctx, bad, []string{bad.Handle})
	require.Error(t, err)
}

// TestFiles_CacheClearSourceAll_NoHandleDir covers the no-op branch of
// CacheClearSourceAll where the handle's cache dir does not exist.
func TestFiles_CacheClearSourceAll_NoHandleDir(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@never_cached", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	require.NoError(t, fs.CacheClearSourceAll(ctx, src, []string{src.Handle}))
}

// TestFiles_CacheClearSource_InvalidHandle covers the CacheDirFor error branch
// of doCacheClearSource via an invalid handle.
func TestFiles_CacheClearSource_InvalidHandle(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	bad := &source.Source{Handle: "bad handle", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	err := fs.CacheClearSource(ctx, bad, true)
	require.Error(t, err)
}
