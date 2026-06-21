package files_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

// mustCSVSrc returns a CSV file source backed by a temp file containing data.
func mustCSVSrc(t *testing.T, dir, data string) *source.Source {
	t.Helper()
	fp := filepath.Join(dir, "data.csv")
	require.NoError(t, os.WriteFile(fp, []byte(data), 0o600))
	return &source.Source{
		Handle:   "@csv_" + stringz.Uniq8(),
		Type:     drivertype.CSV,
		Location: fp,
	}
}

func TestFiles_CacheDirAndTempDir(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	tmpDir := tu.TempDir(t, "temp")
	cacheDir := tu.TempDir(t, "cache")
	fs, err := files.New(ctx, nil, testh.TempLockFunc(t), tmpDir, cacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	require.Equal(t, cacheDir, fs.CacheDir())
	require.Equal(t, tmpDir, fs.TempDir())
}

func TestDefaultCacheDirAndTempDir(t *testing.T) {
	require.NotEmpty(t, files.DefaultCacheDir())
	tmp := files.DefaultTempDir()
	require.NotEmpty(t, tmp)
	require.True(t, strings.HasSuffix(tmp, "sq"), "got %s", tmp)
}

func TestFiles_CacheDirFor(t *testing.T) {
	_, fs := newTestFiles(t)

	// Valid handle.
	src := &source.Source{Handle: "@h1", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	dir, err := fs.CacheDirFor(src)
	require.NoError(t, err)
	require.NotEmpty(t, dir)

	// Stdin handle gets a unique suffix each time.
	stdinSrc := &source.Source{Handle: source.StdinHandle, Type: drivertype.CSV, Location: source.StdinHandle}
	d1, err := fs.CacheDirFor(stdinSrc)
	require.NoError(t, err)
	d2, err := fs.CacheDirFor(stdinSrc)
	require.NoError(t, err)
	require.NotEqual(t, d1, d2, "stdin cache dir must be unique each call")

	// Invalid handle returns an error.
	badSrc := &source.Source{Handle: "not-a-handle", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	_, err = fs.CacheDirFor(badSrc)
	require.Error(t, err)
}

func TestFiles_CachePaths_InvalidHandle(t *testing.T) {
	_, fs := newTestFiles(t)
	badSrc := &source.Source{Handle: "nope", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	_, _, _, err := fs.CachePaths(badSrc)
	require.Error(t, err)
}

// TestFiles_sourceHash_IngestMutate verifies that only options tagged
// TagIngestMutate affect the cache dir hash.
func TestFiles_sourceHash_IngestMutate(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	optMutate := options.NewBool("test.mutate", nil, false,
		"mutate", "mutate help", options.TagIngestMutate)
	optPlain := options.NewBool("test.plain", nil, false,
		"plain", "plain help", options.TagSource)

	optReg := &options.Registry{}
	optReg.Add(optMutate, optPlain)

	fs, err := files.New(ctx, optReg, testh.TempLockFunc(t),
		tu.TempDir(t, "temp"), tu.TempDir(t, "cache"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	base := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	baseDir, err := fs.CacheDirFor(base)
	require.NoError(t, err)

	// A mutate-tagged opt changes the hash.
	withMutate := &source.Source{
		Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv",
		Options: options.Options{optMutate.Key(): true},
	}
	mutateDir, err := fs.CacheDirFor(withMutate)
	require.NoError(t, err)
	require.NotEqual(t, baseDir, mutateDir, "mutate-tagged opt must change cache dir")

	// A non-mutate opt does NOT change the hash.
	withPlain := &source.Source{
		Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv",
		Options: options.Options{optPlain.Key(): true},
	}
	plainDir, err := fs.CacheDirFor(withPlain)
	require.NoError(t, err)
	require.Equal(t, baseDir, plainDir, "non-mutate opt must not change cache dir")
}

func TestFiles_CacheClearAll(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	// Populate the cache dir.
	src := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	leafDir, cacheDB, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	populateLeaf(t, leafDir)
	require.True(t, ioz.FileAccessible(cacheDB))

	require.NoError(t, fs.CacheClearAll(ctx))
	require.False(t, ioz.FileAccessible(cacheDB))
	require.True(t, ioz.DirExists(fs.CacheDir()), "cache dir should be recreated")
}

// TestFiles_CacheClearAll_TempDirUnusable verifies that CacheClearAll does not
// depend on the global temp dir (DefaultTempDir) being usable. The clear
// relocates the cache dir before deleting it; that relocation must happen
// within the cache dir's own filesystem, not via os.TempDir, otherwise it
// fails with EXDEV when the cache and temp dirs are on different filesystems.
//
// The cross-filesystem condition is simulated by pointing TMPDIR at a regular
// file, which makes DefaultTempDir() impossible to create: the old
// implementation relocated there and failed.
func TestFiles_CacheClearAll_TempDirUnusable(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	// Real, writable workspace, created before TMPDIR is clobbered.
	work := t.TempDir()
	cacheDir := filepath.Join(work, "cache")
	tmpDir := filepath.Join(work, "ftmp")
	require.NoError(t, os.MkdirAll(cacheDir, 0o700))
	require.NoError(t, os.MkdirAll(tmpDir, 0o700))
	// Populate the cache dir so the relocate-then-delete path runs.
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "stale"), []byte("x"), 0o600))

	// Point TMPDIR at a regular file so DefaultTempDir() (os.TempDir()/sq)
	// cannot be created. A no-op config lock keeps the cache sweep on Close
	// from depending on TMPDIR too.
	bogusTmp := filepath.Join(work, "tmpfile")
	require.NoError(t, os.WriteFile(bogusTmp, nil, 0o600))
	t.Setenv("TMPDIR", bogusTmp)

	noopLock := func(context.Context) (func(), error) { return func() {}, nil }
	fs, err := files.New(ctx, nil, noopLock, tmpDir, cacheDir)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	require.NoError(t, fs.CacheClearAll(ctx),
		"CacheClearAll must not depend on the global temp dir")
	require.True(t, ioz.DirExists(cacheDir), "cache dir recreated")
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	require.Empty(t, entries, "cache dir emptied")
}

// TestFiles_CacheClearSource verifies both clearDownloads modes.
func TestFiles_CacheClearSource(t *testing.T) {
	for _, clearDownloads := range []bool{true, false} {
		t.Run(tu.Name("clearDownloads", clearDownloads), func(t *testing.T) {
			ctx, fs := newTestFiles(t)
			t.Cleanup(func() { assert.NoError(t, fs.Close()) })

			src := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv"}
			leafDir, cacheDB, _, err := fs.CachePaths(src)
			require.NoError(t, err)
			populateLeaf(t, leafDir)

			// Create a download subdir with a file.
			dlDir := filepath.Join(leafDir, "download")
			require.NoError(t, os.MkdirAll(dlDir, 0o700))
			dlFile := filepath.Join(dlDir, "body")
			require.NoError(t, os.WriteFile(dlFile, []byte("body"), 0o600))

			require.NoError(t, fs.CacheClearSource(ctx, src, clearDownloads))
			require.False(t, ioz.FileAccessible(cacheDB), "cache DB always cleared")

			if clearDownloads {
				require.False(t, ioz.FileAccessible(dlFile), "download cleared")
			} else {
				require.True(t, ioz.FileAccessible(dlFile), "download preserved")
			}
		})
	}
}

// TestFiles_CacheClearSource_NotExist clears a source whose cache dir
// doesn't exist (no-op success).
func TestFiles_CacheClearSource_NotExist(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	src := &source.Source{Handle: "@gone", Type: drivertype.CSV, Location: "/tmp/gone.csv"}
	require.NoError(t, fs.CacheClearSource(ctx, src, true))
}

func TestFiles_CacheLockAcquire(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	unlock, err := fs.CacheLockAcquire(ctx, src)
	require.NoError(t, err)
	require.NotNil(t, unlock)
	unlock()

	// Invalid handle error path.
	badSrc := &source.Source{Handle: "bad handle", Type: drivertype.CSV, Location: "/tmp/a.csv"}
	_, err = fs.CacheLockAcquire(ctx, badSrc)
	require.Error(t, err)
}

// TestFiles_WriteIngestChecksum_CreatesLeafDir verifies that
// WriteIngestChecksum creates the cache leaf dir when it doesn't already
// exist, rather than failing because checksum.WriteFile can't open a file in
// a missing directory.
func TestFiles_WriteIngestChecksum_CreatesLeafDir(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	dir := tu.TempDir(t, "src")
	src := mustCSVSrc(t, dir, "a,b,c\n1,2,3\n")
	backingSrc := &source.Source{Handle: src.Handle + "_cached", Type: drivertype.SQLite}

	// Deliberately do NOT create the cache leaf dir first.
	leafDir, _, checksumsPath, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.False(t, ioz.DirExists(leafDir), "precondition: leaf dir absent")

	require.NoError(t, fs.WriteIngestChecksum(ctx, src, backingSrc),
		"WriteIngestChecksum must create the cache leaf dir if absent")
	require.True(t, ioz.FileAccessible(checksumsPath), "checksums file written")
}

// TestFiles_WriteIngestChecksum_File covers WriteIngestChecksum and
// CachedBackingSourceFor for a local file source.
func TestFiles_WriteIngestChecksum_File(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	dir := tu.TempDir(t, "src")
	src := mustCSVSrc(t, dir, "a,b,c\n1,2,3\n")
	backingSrc := &source.Source{Handle: src.Handle + "_cached", Type: drivertype.SQLite}

	// Before writing checksum, no cached backing source.
	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, fs.WriteIngestChecksum(ctx, src, backingSrc))

	_, _, checksumsPath, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.True(t, ioz.FileAccessible(checksumsPath), "checksums file written")

	// Still no cache.sqlite.db, so ok=false.
	_, ok, err = fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok)

	// Now create the cache DB file.
	_, cacheDB, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cacheDB, []byte("fake"), 0o600))

	got, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.True(t, ok, "checksum matches and cache DB exists")
	require.Equal(t, drivertype.SQLite, got.Type)
	require.True(t, strings.HasPrefix(got.Location, "sqlite3://"))

	// Mutate the source file: checksum no longer matches.
	require.NoError(t, os.WriteFile(src.Location, []byte("a,b,c\n9,9,9\n"), 0o600))
	_, ok, err = fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok, "checksum mismatch -> not cached")
}

// TestFiles_CachedBackingSourceFor_UnsupportedType verifies the default
// branch errors for a SQL source.
func TestFiles_CachedBackingSourceFor_UnsupportedType(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	sqlSrc := &source.Source{
		Handle:   "@pg",
		Type:     drivertype.Pg,
		Location: "postgres://user:pass@localhost/db",
	}
	_, ok, err := fs.CachedBackingSourceFor(ctx, sqlSrc)
	require.Error(t, err)
	require.False(t, ok)
}

// TestFiles_WriteIngestChecksum_HTTP covers the HTTP branch of
// WriteIngestChecksum and the remote-file cached backing source path.
func TestFiles_WriteIngestChecksum_HTTP(t *testing.T) {
	const body = "a,b,c\n1,2,3\n"
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}
	backingSrc := &source.Source{Handle: src.Handle + "_cached", Type: drivertype.SQLite}

	// Drive a download so the body is cached on disk.
	r, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	_, err = readAllAndClose(t, r)
	require.NoError(t, err)

	// A second read ensures the completed download is registered in the
	// downloadedFiles map (the "already downloaded" fast path).
	r2, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	_, err = readAllAndClose(t, r2)
	require.NoError(t, err)

	// Exercises the HTTP branch of WriteIngestChecksum (stream Filled/Done).
	require.NoError(t, fs.WriteIngestChecksum(ctx, src, backingSrc))

	// CachedBackingSourceFor for the remote file: checksum present but no
	// cache DB yet -> ok=false. This drives cachedBackingSourceForRemoteFile
	// -> maybeStartDownload(already downloaded) -> cachedBackingSourceForFile.
	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.NoError(t, err)
	require.False(t, ok)
}

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

// TestFiles_CachedBackingSourceFor_RemoteBadURL covers the error path of
// cachedBackingSourceForRemoteFile: starting the download fails because the
// URL is malformed (rejected by downloader.New).
func TestFiles_CachedBackingSourceFor_RemoteBadURL(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: "http://%zz"}
	_, ok, err := fs.CachedBackingSourceFor(ctx, src)
	require.Error(t, err)
	require.False(t, ok)
}
