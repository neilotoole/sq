package files_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
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
// TestFiles_NewReader_HTTP_StreamReuseSeal in files_extra2_test.go.)
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
// TestFiles_Filesize_HTTP_ActiveStream in files_extra2_test.go, which
// synchronize the draining goroutine via a WaitGroup.

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
