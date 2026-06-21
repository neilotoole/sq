// This file contains white-box tests targeting specific hard-to-reach branches
// in cache.go and downloader.go: the non-GET path of cache.paths, the
// empty-header and missing-body branches of exists/checksumsMatch, the
// cacheFileOnError guards, the state() error branches, and the filesystem-error
// cleanup paths of writeHeader and newResponseCacher (driven via a read-only
// parent directory).

package downloader

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

// TestCache_paths_nonGet covers the non-GET branch of cache.paths, which
// prefixes the file names with the HTTP method.
func TestCache_paths_nonGet(t *testing.T) {
	c := &cache{dir: "/tmp/dlcache"}
	req, err := http.NewRequest(http.MethodHead, "http://example.com", nil)
	require.NoError(t, err)

	header, body, sum := c.paths(req)
	require.True(t, strings.HasSuffix(header, filepath.Join("main", "HEAD_header")))
	require.True(t, strings.HasSuffix(body, filepath.Join("main", "HEAD_body")))
	require.True(t, strings.HasSuffix(sum, filepath.Join("main", "HEAD_checksums.txt")))
}

// TestCache_exists_emptyHeader covers the fi.Size()==0 branch of exists: all
// three cache files exist and checksums match, but the header file is empty.
func TestCache_exists_emptyHeader(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))

	// Empty header file (size 0), but present so it's not "inconsistent".
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte{}, 0o600))
	fpBody := filepath.Join(mainDir, "body")
	require.NoError(t, os.WriteFile(fpBody, []byte("body"), 0o600))
	sum, err := checksum.ForFile(fpBody)
	require.NoError(t, err)
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"), sum, "body"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	require.False(t, c.exists(req))
}

// TestCache_checksumsMatch_missingBody covers the ForFile-error branch of
// checksumsMatch: the checksum file exists and parses, but the body file is
// absent, so computing a fresh checksum fails.
func TestCache_checksumsMatch_missingBody(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	// Only a checksums file; no body file.
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"),
		checksum.Checksum("1111111111111111111111111111111111111111111111111111111111111111"), "body"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	// Discard the sum result (it's always empty on failure) so the source's
	// //nolint:unparam directive on checksumsMatch stays meaningful.
	_, ok := c.checksumsMatch(req)
	require.False(t, ok)
}

// TestCacheFileOnError_guards covers the early-return guards of
// Downloader.cacheFileOnError: nil request and nil cache both yield "".
func TestCacheFileOnError_guards(t *testing.T) {
	dl := &Downloader{continueOnError: true}

	// nil request guard.
	require.Empty(t, dl.cacheFileOnError(nil, nil))

	// nil cache guard (cache not set).
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	require.Empty(t, dl.cacheFileOnError(req, nil))
}

// TestState_headerOpenError covers the state() branch where the header file
// can't be opened (here, because "header" is a directory, not a file). exists()
// must first report true, so we make a valid cache and then replace the header
// with a directory — but that would fail checksum. Instead we drive the
// ReadResponseHeader-error branch: a valid cache whose header contains data
// that exists() accepts (non-empty, checksum over body matches) but that
// httpz.ReadResponseHeader can't parse.
func TestState_readHeaderError(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	dlDir := t.TempDir()
	c := &cache{dir: dlDir}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))

	// Non-empty header with garbage that ReadResponseHeader rejects.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte("not a valid http header line\r\n"), 0o600))
	fpBody := filepath.Join(mainDir, "body")
	require.NoError(t, os.WriteFile(fpBody, []byte("body"), 0o600))
	sum, err := checksum.ForFile(fpBody)
	require.NoError(t, err)
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"), sum, "body"))

	dl := &Downloader{
		name:  t.Name(),
		url:   "http://example.com",
		dlDir: dlDir,
		cache: c,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	// exists() is true (non-empty header, matching checksum), but
	// ReadResponseHeader fails -> Uncached.
	require.True(t, c.exists(req))
	require.Equal(t, Uncached, dl.state(req))
}

// TestCache_writeHeader_dirError covers the RequireDir/WriteToFile error branch
// of writeHeader by making the cache dir's parent read-only so the "main"
// subdirectory can't be created. Skipped on Windows (chmod semantics differ)
// and when running as root (root bypasses permission checks).
func TestCache_writeHeader_dirError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	parent := t.TempDir()
	cacheDir := filepath.Join(parent, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	// Make the cache dir read-only so creating "main" inside it fails.
	require.NoError(t, os.Chmod(cacheDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	c := &cache{dir: cacheDir}
	raw := "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBufferString(raw)), nil)
	require.NoError(t, err)

	err = c.writeHeader(ctx, resp)
	require.Error(t, err)
}

// TestCache_newResponseCacher_dirError covers the RequireDir error cleanup
// branch of newResponseCacher (the "staging" dir can't be created), and
// verifies that the response body is closed on failure.
func TestCache_newResponseCacher_dirError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	parent := t.TempDir()
	cacheDir := filepath.Join(parent, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	require.NoError(t, os.Chmod(cacheDir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(cacheDir, 0o755) })

	c := &cache{dir: cacheDir}
	body := &closeTrackingReader{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body:       body,
	}

	rc, err := c.newResponseCacher(ctx, resp)
	require.Error(t, err)
	require.Nil(t, rc)
	require.True(t, body.closed, "response body should be closed on setup failure")
}

// closeTrackingReader is an io.ReadCloser that records whether Close was called.
type closeTrackingReader struct {
	closed bool
}

func (r *closeTrackingReader) Read([]byte) (int, error) { return 0, nil }

func (r *closeTrackingReader) Close() error {
	r.closed = true
	return nil
}
