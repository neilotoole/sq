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

	// The real ingest flow creates the cache leaf dir (via the cache lock)
	// before writing the checksum; replicate that here.
	leafDir, _, _, err := fs.CachePaths(src)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(leafDir, 0o700))

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
