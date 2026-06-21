package files_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// readAllAndClose reads r to completion and closes it.
func readAllAndClose(t *testing.T, r io.ReadCloser) ([]byte, error) {
	t.Helper()
	b, err := io.ReadAll(r)
	assert.NoError(t, r.Close())
	return b, err
}

// newCSVServer returns an httptest server serving body as CSV.
func newCSVServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)
	return srvr
}

// TestFiles_NewReader_HTTP drives maybeStartDownload -> downloaderFor ->
// httpClientFor via NewReader, then exercises the already-downloaded path.
func TestFiles_NewReader_HTTP(t *testing.T) {
	const body = "a,b,c\n1,2,3\n4,5,6\n"
	srvr := newCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	// First read: stream download path.
	r, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	got, err := readAllAndClose(t, r)
	require.NoError(t, err)
	require.Equal(t, body, string(got))

	// Second read: already-downloaded path.
	r2, err := fs.NewReader(ctx, src, false)
	require.NoError(t, err)
	got2, err := readAllAndClose(t, r2)
	require.NoError(t, err)
	require.Equal(t, body, string(got2))
}

// TestFiles_NewReader_HTTP_Final exercises the finalRdr seal path when
// newReader starts a fresh download: maybeStartDownload returns a new stream,
// which is then sealed because finalRdr is true. (The distinct branch where an
// already in-progress stream is reused and sealed is covered by
// TestFiles_NewReader_HTTP_StreamReuseSeal.)
func TestFiles_NewReader_HTTP_Final(t *testing.T) {
	const body = "x,y\n1,2\n"
	srvr := newCSVServer(t, body)

	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}

	r, err := fs.NewReader(ctx, src, true)
	require.NoError(t, err)
	got, err := readAllAndClose(t, r)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
}

// TestFiles_NewReader_ErrorTypes covers the TypeSQL error branch of newReader
// and the missing-file error path.
func TestFiles_NewReader_ErrorTypes(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	t.Run("sql", func(t *testing.T) {
		src := &source.Source{Handle: "@pg", Type: drivertype.Pg, Location: "postgres://u:p@localhost/db"}
		_, err := fs.NewReader(ctx, src, false)
		require.Error(t, err)
	})

	t.Run("missing_file", func(t *testing.T) {
		src := &source.Source{Handle: "@x", Location: "/no/such/file.csv"}
		_, err := fs.NewReader(ctx, src, false)
		require.Error(t, err)
	})
}

// Note: the HTTP branches of Filesize (already-downloaded fast path and
// blocking on an in-progress download stream's Total) are covered by
// TestFiles_Filesize_HTTP_AlreadyDownloaded and
// TestFiles_Filesize_HTTP_ActiveStream, which synchronize the draining
// goroutine via a WaitGroup.

// TestFiles_Filesize_ErrorTypes covers the SQL error branch of Filesize and
// the missing local-file error path.
func TestFiles_Filesize_ErrorTypes(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	sqlSrc := &source.Source{Handle: "@pg", Type: drivertype.Pg, Location: "postgres://u:p@localhost/db"}
	_, err := fs.Filesize(ctx, sqlSrc)
	require.Error(t, err)

	missingSrc := &source.Source{Handle: "@x", Location: "/no/such/file.csv"}
	_, err = fs.Filesize(ctx, missingSrc)
	require.Error(t, err)
}

// TestFiles_Ping covers all Ping branches.
func TestFiles_Ping(t *testing.T) {
	ctx, fs := newTestFiles(t)
	t.Cleanup(func() { assert.NoError(t, fs.Close()) })

	t.Run("stdin", func(t *testing.T) {
		src := &source.Source{Handle: source.StdinHandle, Location: source.StdinHandle}
		require.NoError(t, fs.Ping(ctx, src))
	})

	t.Run("file_exists", func(t *testing.T) {
		dir := tu.TempDir(t, "ping")
		src := mustCSVSrc(t, dir, "a,b\n1,2\n")
		require.NoError(t, fs.Ping(ctx, src))
	})

	t.Run("file_missing", func(t *testing.T) {
		src := &source.Source{Handle: "@h", Type: drivertype.CSV, Location: "/no/such/file.csv"}
		require.Error(t, fs.Ping(ctx, src))
	})

	t.Run("http_ok", func(t *testing.T) {
		srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodHead, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srvr.Close)
		src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}
		require.NoError(t, fs.Ping(ctx, src))
	})

	t.Run("http_non200", func(t *testing.T) {
		srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(srvr.Close)
		src := &source.Source{Handle: "@remote", Type: drivertype.CSV, Location: srvr.URL}
		require.Error(t, fs.Ping(ctx, src))
	})

	t.Run("default", func(t *testing.T) {
		src := &source.Source{Handle: "@pg", Type: drivertype.Pg, Location: "postgres://u:p@localhost/db"}
		require.Error(t, fs.Ping(ctx, src))
	})
}

func TestFiles_CreateTemp(t *testing.T) {
	t.Run("clean_false", func(t *testing.T) {
		_, fs := newTestFiles(t)
		f, err := fs.CreateTemp("test-*.tmp", false)
		require.NoError(t, err)
		name := f.Name()
		require.NoError(t, f.Close())
		require.True(t, strings.HasPrefix(name, fs.TempDir()))
		require.FileExists(t, name)
		require.NoError(t, fs.Close())
		// clean=false: file not removed by Close (but TempDir is removed).
	})

	t.Run("clean_true", func(t *testing.T) {
		_, fs := newTestFiles(t)
		f, err := fs.CreateTemp("test-*.tmp", true)
		require.NoError(t, err)
		name := f.Name()
		require.NoError(t, f.Close())
		require.FileExists(t, name)
		require.NoError(t, fs.Close())
		require.NoFileExists(t, name, "clean=true file removed on Close")
	})
}

// newStringReader returns an io.Reader over s.
func newStringReader(s string) io.Reader {
	return strings.NewReader(s)
}

// newDuckHeaderReader returns a reader over a minimal DuckDB file header:
// the magic bytes "DUCK" at offset 8, padded to fill the magic-number probe.
func newDuckHeaderReader() io.Reader {
	b := make([]byte, 261)
	copy(b[8:12], []byte("DUCK"))
	return strings.NewReader(string(b))
}

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
