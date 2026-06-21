package files_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

// newCacheableCSVServer returns an httptest server serving body as CSV with a
// Cache-Control header, so the download cache treats the body as fresh and
// re-serves it from disk on subsequent fetches. This is what drives the
// already-downloaded-file branches (downloadedFiles map population).
func newCacheableCSVServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Cache-Control", "max-age=300")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)
	return srvr
}

// newFilesAtDir constructs a *files.Files instance using the given temp and
// cache dirs, so that two instances can share an on-disk download cache.
func newFilesAtDir(t *testing.T, tmpDir, cacheDir string) (context.Context, *files.Files) {
	t.Helper()
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	optReg := &options.Registry{}
	optReg.Add(files.OptCacheLockTimeout)
	fs, err := files.New(ctx, optReg, testh.TempLockFunc(t), tmpDir, cacheDir)
	require.NoError(t, err)
	return ctx, fs
}

// TestFiles_Filesize_HTTP_AlreadyDownloaded exercises the Filesize HTTP branch
// where the download is already registered in the downloadedFiles map (i.e. a
// fresh Files instance serving a body cached on disk by a prior instance).
func TestFiles_Filesize_HTTP_AlreadyDownloaded(t *testing.T) {
	const body = "a,b,c\n1,2,3\n4,5,6\n"
	srvr := newCacheableCSVServer(t, body)

	cacheDir := tu.TempDir(t, "cache")
	tmpDir := tu.TempDir(t, "temp")
	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	// Instance 1: download the body to the shared on-disk cache.
	ctx1, fs1 := newFilesAtDir(t, tmpDir, cacheDir)
	r, err := fs1.NewReader(ctx1, src, false)
	require.NoError(t, err)
	got, err := readAllAndClose(t, r)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
	require.NoError(t, fs1.Close())

	// Instance 2 shares the cache dir. Its first NewReader hits the cached
	// body on disk, which populates the downloadedFiles map.
	ctx2, fs2 := newFilesAtDir(t, tmpDir, cacheDir)
	t.Cleanup(func() { assert.NoError(t, fs2.Close()) })
	r2, err := fs2.NewReader(ctx2, src, false)
	require.NoError(t, err)
	got2, err := readAllAndClose(t, r2)
	require.NoError(t, err)
	require.Equal(t, body, string(got2))

	// Now the downloadedFiles map is populated, so Filesize serves the size
	// straight from disk.
	size, err := fs2.Filesize(ctx2, src)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), size)

	// A subsequent NewReader opens the cached file from disk directly.
	r3, err := fs2.NewReader(ctx2, src, false)
	require.NoError(t, err)
	got3, err := readAllAndClose(t, r3)
	require.NoError(t, err)
	require.Equal(t, body, string(got3))
}

// TestFiles_Filesize_HTTP_ActiveStream exercises the Filesize HTTP branch that
// blocks on the in-progress download stream's Total. The reader must be drained
// concurrently for Total to complete.
func TestFiles_Filesize_HTTP_ActiveStream(t *testing.T) {
	const body = "a,b,c\n1,2,3\n4,5,6\n7,8,9\n"
	srvr := newCacheableCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	// Open a reader to start the download stream, but don't drain it yet.
	r, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = readAllAndClose(t, r)
	})

	// Filesize blocks on the stream's Total, which completes once the reader
	// above drains the stream to EOF.
	size, err := fs.Filesize(ctx, src)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), size)
	wg.Wait()
}

// TestFiles_Filesize_SQLError covers the SQL-source error branch of Filesize.
func TestFiles_Filesize_SQLError(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@pg", Type: drivertype.Pg, Location: "postgres://x:y@localhost/db"}
	_, err := fs.Filesize(ctx, src)
	require.Error(t, err)
}

// TestFiles_NewReader_Stdin covers the stdin branches of newReader: the
// not-cached error, and the finalRdr seal path.
func TestFiles_NewReader_Stdin(t *testing.T) {
	t.Run("not_cached", func(t *testing.T) {
		ctx, fs := newTestFiles(t)
		t.Cleanup(func() { assert.NoError(t, fs.Close()) })
		src := &source.Source{Handle: source.StdinHandle, Location: source.StdinHandle}
		_, err := fs.NewReader(ctx, src, false)
		require.Error(t, err, "@stdin not added -> error")
	})

	t.Run("seal", func(t *testing.T) {
		th := testh.New(t)
		fs := th.Files()

		fpath := tu.WriteTemp(t, "stdin-*.csv", []byte("a,b\n1,2\n"), false)
		fh, err := os.Open(fpath)
		require.NoError(t, err)
		require.NoError(t, fs.AddStdin(th.Context, fh)) // AddStdin closes fh

		src := &source.Source{Handle: source.StdinHandle, Location: source.StdinHandle}
		// finalRdr=true seals the stdin stream.
		r, err := fs.NewReader(th.Context, src, true)
		require.NoError(t, err)
		got, err := readAllAndClose(t, r)
		require.NoError(t, err)
		require.Equal(t, "a,b\n1,2\n", string(got))
	})
}

// TestFiles_NewReader_HTTP_StreamReuseSeal drives the in-progress download
// stream reuse + seal branch of newReader: a download stream is started, then
// a second NewReader with finalRdr=true reuses and seals it.
func TestFiles_NewReader_HTTP_StreamReuseSeal(t *testing.T) {
	const body = "a,b,c\n1,2,3\n"
	srvr := newCacheableCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })
	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	// First NewReader starts the download stream (don't drain).
	r1, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)

	// Second NewReader reuses the in-progress stream and seals it.
	r2, err := fs.NewReader(ctx, src, true)
	require.NoError(t, err)

	got1, err := readAllAndClose(t, r1)
	require.NoError(t, err)
	require.Equal(t, body, string(got1))

	got2, err := readAllAndClose(t, r2)
	require.NoError(t, err)
	require.Equal(t, body, string(got2))
}
