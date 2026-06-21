// This file contains additional white-box tests for the cache implementation
// in cache.go. It uses package downloader (not downloader_test) to construct
// cache and responseCacher literals directly, and to call unexported methods
// that are otherwise hard to reach via the public Downloader API.
//
// These tests focus on the cache-consistency, cache-read, checksum, and
// responseCacher cleanup branches that aren't covered by the higher-level
// download lifecycle tests.

package downloader

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

// newCacheTestReq returns a GET request bound to ctx, suitable for driving
// the unexported cache methods. The URL is irrelevant because the cache keys
// off the cache dir, not the request URL.
func newCacheTestReq(t *testing.T, ctx context.Context) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.invalid", nil)
	require.NoError(t, err)
	return req
}

// writeValidCache writes a fully consistent main cache (header, body,
// checksums.txt) under c.dir.
func writeValidCache(t *testing.T, c *cache, body string) {
	t.Helper()
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))

	// A minimal but valid serialized HTTP response header (no body), as
	// produced by httputil.DumpResponse(resp, false): status line plus a
	// blank line. http.ReadResponse will then read the body from the
	// concatenated body file.
	header := "HTTP/1.1 200 OK\r\nContent-Length: " +
		strconv.Itoa(len(body)) + "\r\n\r\n"
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte(header), 0o600))

	fpBody := filepath.Join(mainDir, "body")
	require.NoError(t, os.WriteFile(fpBody, []byte(body), 0o600))

	sum, err := checksum.ForFile(fpBody)
	require.NoError(t, err)
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"), sum, "body"))
}

// TestCache_exists_inconsistent_missingFiles verifies that exists() returns
// false and clears the cache when the main dir contains some-but-not-all of
// the three required cache files (here, only a header).
func TestCache_exists_inconsistent_missingFiles(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	// Only the header file present: inconsistent.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte("HTTP/1.1 200 OK\r\n\r\n"), 0o600))

	req := newCacheTestReq(t, ctx)
	require.False(t, c.exists(req))

	// The inconsistent cache should have been cleared: the header file is gone.
	_, err := os.Stat(filepath.Join(mainDir, "header"))
	require.True(t, errors.Is(err, os.ErrNotExist))
}

// TestCache_exists_inconsistent_checksumMismatch verifies that exists() clears
// the cache when all three files are present but the stored checksum doesn't
// match the actual body bytes.
func TestCache_exists_inconsistent_checksumMismatch(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte("HTTP/1.1 200 OK\r\n\r\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "body"), []byte("actual body"), 0o600))
	// Write a checksum that deliberately does not match the body.
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"),
		checksum.Checksum("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"), "body"))

	req := newCacheTestReq(t, ctx)
	require.False(t, c.exists(req))

	// Cache cleared.
	_, err := os.Stat(filepath.Join(mainDir, "body"))
	require.True(t, errors.Is(err, os.ErrNotExist))
}

// TestCache_exists_valid verifies the happy path: a fully consistent cache
// reports exists()==true.
func TestCache_exists_valid(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	writeValidCache(t, c, "hello body")
	req := newCacheTestReq(t, ctx)
	require.True(t, c.exists(req))
}

// TestCache_exists_noMainDir verifies exists() returns false when the cache
// directory has never been written (no main dir at all).
func TestCache_exists_noMainDir(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	req := newCacheTestReq(t, ctx)
	require.False(t, c.exists(req))
}

// TestCache_clearIfInconsistent_emptyMainDir verifies that an empty main dir
// is considered consistent (no-op, returns nil).
func TestCache_clearIfInconsistent_emptyMainDir(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	require.NoError(t, ioz.RequireDir(filepath.Join(c.dir, "main")))
	req := newCacheTestReq(t, ctx)
	require.NoError(t, c.clearIfInconsistent(req))
}

// TestCache_get_headerMissing verifies that get() returns (nil, nil) when the
// header file isn't present.
func TestCache_get_headerMissing(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	req := newCacheTestReq(t, ctx)
	resp, err := c.get(ctx, req)
	require.NoError(t, err)
	require.Nil(t, resp)
}

// TestCache_get_checksumMismatch verifies that get() returns (nil, nil) when
// the header exists but the stored checksum doesn't match the body.
func TestCache_get_checksumMismatch(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte("HTTP/1.1 200 OK\r\n\r\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "body"), []byte("body"), 0o600))
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"),
		checksum.Checksum("0000000000000000000000000000000000000000000000000000000000000000"), "body"))

	req := newCacheTestReq(t, ctx)
	resp, err := c.get(ctx, req)
	require.NoError(t, err)
	require.Nil(t, resp)
}

// TestCache_get_valid verifies that get() reconstructs a readable response from
// a valid hand-built cache, and that the body matches the cached bytes.
func TestCache_get_valid(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	const body = "hello body"
	writeValidCache(t, c, body)

	req := newCacheTestReq(t, ctx)
	resp, err := c.get(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
}

// TestCache_get_malformedHeader verifies that get() returns an error when the
// header file contains data that http.ReadResponse can't parse.
func TestCache_get_malformedHeader(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))

	// Garbage that is not a valid HTTP response status line.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "header"), []byte("this is not http\r\n\r\n"), 0o600))
	fpBody := filepath.Join(mainDir, "body")
	require.NoError(t, os.WriteFile(fpBody, []byte("body"), 0o600))
	sum, err := checksum.ForFile(fpBody)
	require.NoError(t, err)
	require.NoError(t, checksum.WriteFile(filepath.Join(mainDir, "checksums.txt"), sum, "body"))

	req := newCacheTestReq(t, ctx)
	resp, err := c.get(ctx, req)
	require.Error(t, err)
	require.Nil(t, resp)
}

// TestCache_cachedChecksum_missingFile verifies ("", false) when checksums.txt
// is absent.
func TestCache_cachedChecksum_missingFile(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	req := newCacheTestReq(t, ctx)
	sum, ok := c.cachedChecksum(req)
	require.False(t, ok)
	require.Empty(t, sum)
}

// TestCache_cachedChecksum_unparseable verifies ("", false) when checksums.txt
// contains garbage that can't be parsed.
func TestCache_cachedChecksum_unparseable(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	// checksum.ReadFile expects lines of "<sum>  <name>"; a line with only one
	// field fails to parse.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "checksums.txt"), []byte("garbage-no-name\n"), 0o600))

	req := newCacheTestReq(t, ctx)
	sum, ok := c.cachedChecksum(req)
	require.False(t, ok)
	require.Empty(t, sum)
}

// TestCache_cachedChecksum_multipleEntries verifies ("", false) when the
// checksums file contains more than one entry (len(sums) != 1).
func TestCache_cachedChecksum_multipleEntries(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	fp := filepath.Join(mainDir, "checksums.txt")
	// Two valid entries (different names) -> len(sums) == 2.
	require.NoError(t, checksum.WriteFile(fp,
		checksum.Checksum("1111111111111111111111111111111111111111111111111111111111111111"), "body"))
	f, err := os.OpenFile(fp, os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	require.NoError(t, checksum.Write(f,
		checksum.Checksum("2222222222222222222222222222222222222222222222222222222222222222"), "other"))
	require.NoError(t, f.Close())

	req := newCacheTestReq(t, ctx)
	sum, ok := c.cachedChecksum(req)
	require.False(t, ok)
	require.Empty(t, sum)
}

// TestCache_writeHeader verifies that writeHeader serializes a synthetic
// response's headers to main/header.
func TestCache_writeHeader(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}

	raw := "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 0\r\n\r\n"
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBufferString(raw)), nil)
	require.NoError(t, err)

	require.NoError(t, c.writeHeader(ctx, resp))

	fpHeader := filepath.Join(c.dir, "main", "header")
	got, err := os.ReadFile(fpHeader)
	require.NoError(t, err)
	require.Contains(t, string(got), "200 OK")
}

// TestResponseCacher_ReadAfterClose verifies that calling Read after Close
// returns the "response cache already closed" sentinel error. responseCacher
// is unexported, so we construct it directly with a real staging file.
func TestResponseCacher_ReadAfterClose(t *testing.T) {
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))
	f, err := os.Create(filepath.Join(stagingDir, "body"))
	require.NoError(t, err)

	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       io.NopCloser(bytes.NewBufferString("data")),
		f:          f,
	}

	require.NoError(t, rc.Close())

	n, err := rc.Read(make([]byte, 4))
	require.Error(t, err)
	require.Zero(t, n)
	require.Contains(t, err.Error(), "response cache already closed")
}

// TestResponseCacher_CloseIdempotent verifies that Close is idempotent and
// returns the same error value on the second call.
func TestResponseCacher_CloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))
	f, err := os.Create(filepath.Join(stagingDir, "body"))
	require.NoError(t, err)

	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       io.NopCloser(bytes.NewBufferString("data")),
		f:          f,
	}

	err1 := rc.Close()
	err2 := rc.Close()
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Equal(t, err1, err2)
}

// errReader is an io.ReadCloser whose Read always returns a non-EOF error,
// used to drive responseCacher's mid-body error cleanup branch.
type errReader struct {
	err    error
	closed bool
}

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

func (r *errReader) Close() error {
	r.closed = true
	return nil
}

// TestResponseCacher_ReadBodyError verifies that when the wrapped body returns
// a non-EOF error, Read cleans up (closes body and staging file, removes
// staging dir) and propagates the error.
func TestResponseCacher_ReadBodyError(t *testing.T) {
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))
	fpBody := filepath.Join(stagingDir, "body")
	f, err := os.Create(fpBody)
	require.NoError(t, err)

	body := &errReader{err: errors.New("boom")}
	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       body,
		f:          f,
	}

	n, err := rc.Read(make([]byte, 8))
	require.Error(t, err)
	require.Zero(t, n)
	require.Contains(t, err.Error(), "boom")
	require.True(t, body.closed, "body should have been closed during cleanup")

	// Staging dir should have been removed.
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_cacheAppend_writeError verifies the cacheAppend cleanup
// branch: when the staging file write fails (here, because the file handle is
// already closed), Read returns the wrapped append error and cleans up.
func TestResponseCacher_cacheAppend_writeError(t *testing.T) {
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))
	fpBody := filepath.Join(stagingDir, "body")
	f, err := os.Create(fpBody)
	require.NoError(t, err)
	// Close the file handle so the subsequent Write inside cacheAppend fails.
	require.NoError(t, f.Close())

	body := io.NopCloser(bytes.NewBufferString("data"))
	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       body,
		f:          f,
	}

	n, err := rc.Read(make([]byte, 4))
	require.Error(t, err)
	// Some bytes may have been read from body before the append write failed.
	require.Contains(t, err.Error(), "failed to append")
	_ = n

	// Staging dir removed by cleanup.
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_Read_fullPromote verifies the full happy-path promotion:
// reading to EOF writes the staging cache and atomically promotes it to main,
// producing a valid, re-readable cache.
func TestResponseCacher_Read_fullPromote(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	dir := t.TempDir()
	c := &cache{dir: dir}

	const body = "promote me"
	raw := "HTTP/1.1 200 OK\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewBufferString(raw)), nil)
	require.NoError(t, err)

	rc, err := c.newResponseCacher(ctx, resp)
	require.NoError(t, err)

	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
	require.NoError(t, rc.Close())

	// The main cache should now exist and be valid.
	req := newCacheTestReq(t, ctx)
	require.True(t, c.exists(req))
}

// newStagingCacher builds a responseCacher backed by a real staging dir with a
// "body" file, plus a main dir path, for white-box promotion tests. The caller
// supplies the wrapped body.
func newStagingCacher(t *testing.T, body io.ReadCloser) (rc *responseCacher, stagingDir string) {
	t.Helper()
	dir := t.TempDir()
	stagingDir = filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))

	f, err := os.Create(filepath.Join(stagingDir, "body"))
	require.NoError(t, err)

	rc = &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    filepath.Join(dir, "main"),
		body:       body,
		f:          f,
	}
	return rc, stagingDir
}

// errCloser is an io.ReadCloser whose Read returns EOF and whose Close returns
// a configurable error, used to drive responseCacher.cachePromote's
// body-close-error branch.
type errCloser struct {
	closeErr error
}

func (c *errCloser) Read([]byte) (int, error) { return 0, io.EOF }

func (c *errCloser) Close() error { return c.closeErr }

// TestResponseCacher_cachePromote_statError covers the r.f.Stat() error branch
// (cache.go 622-624) and the deferred staging cleanup (615-618): the staging
// body file handle is closed before cachePromote runs, so Stat fails.
func TestResponseCacher_cachePromote_statError(t *testing.T) {
	rc, stagingDir := newStagingCacher(t, io.NopCloser(bytes.NewBufferString("data")))

	// Close the staging file handle so r.f.Stat() fails inside cachePromote.
	require.NoError(t, rc.f.Close())

	err := rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to stat staging cache body file")

	// Deferred cleanup removed the staging dir and cleared the field.
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_cachePromote_bodyCloseError covers the r.body.Close()
// error branch (cache.go 635-637): Stat and f.Close succeed, but the wrapped
// body's Close returns an error.
func TestResponseCacher_cachePromote_bodyCloseError(t *testing.T) {
	wantErr := errors.New("body close boom")
	rc, stagingDir := newStagingCacher(t, &errCloser{closeErr: wantErr})

	err := rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to close http response body")

	// f was closed and cleared; staging dir removed by deferred cleanup.
	require.Nil(t, rc.f)
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestResponseCacher_cachePromote_renameError covers the ioz.RenameDir error
// branch (cache.go 649-651): all earlier steps succeed (checksum written), but
// the main dir's parent is read-only, so the staging->main rename fails. The
// deferred cleanup then removes the staging dir.
//
// Skipped on Windows (chmod semantics differ) and as root (bypasses perms).
func TestResponseCacher_cachePromote_renameError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission test not reliable on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses filesystem permission checks")
	}

	dir := t.TempDir()
	stagingDir := filepath.Join(dir, "staging")
	require.NoError(t, ioz.RequireDir(stagingDir))

	fpBody := filepath.Join(stagingDir, "body")
	f, err := os.Create(fpBody)
	require.NoError(t, err)
	_, err = f.WriteString("payload")
	require.NoError(t, err)

	// mainDir lives under a read-only parent so RenameDir (an os.Rename into
	// that parent) fails.
	roParent := filepath.Join(dir, "ro")
	require.NoError(t, os.MkdirAll(roParent, 0o755))
	mainDir := filepath.Join(roParent, "main")
	require.NoError(t, os.Chmod(roParent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(roParent, 0o755) })

	rc := &responseCacher{
		log:        lgt.New(t),
		stagingDir: stagingDir,
		mainDir:    mainDir,
		body:       io.NopCloser(bytes.NewBufferString("payload")),
		f:          f,
	}

	err = rc.cachePromote()
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to write download cache")

	// Deferred cleanup removed the staging dir.
	require.Empty(t, rc.stagingDir)
	_, statErr := os.Stat(stagingDir)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

// TestCache_checksumsMatch_noCachedChecksum covers the cachedChecksum-miss
// branch of checksumsMatch (cache.go 304-306): a body file exists but there's
// no checksums.txt, so cachedChecksum returns ("", false).
func TestCache_checksumsMatch_noCachedChecksum(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	c := &cache{dir: t.TempDir()}
	mainDir := filepath.Join(c.dir, "main")
	require.NoError(t, ioz.RequireDir(mainDir))
	// Body present, but no checksums.txt.
	require.NoError(t, os.WriteFile(filepath.Join(mainDir, "body"), []byte("body"), 0o600))

	req := newCacheTestReq(t, ctx)
	// Discard the sum (always empty on failure) so the source's //nolint:unparam
	// on checksumsMatch stays meaningful, matching TestCache_checksumsMatch_missingBody.
	_, ok := c.checksumsMatch(req)
	require.False(t, ok)
}

// TestCache_clear_removeError covers the error branch of clear() (cache.go
// 341-344): the cache dir's parent is read-only, so os.RemoveAll on the cache
// dir fails. Skipped on Windows and as root.
func TestCache_clear_removeError(t *testing.T) {
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
	// Put a file inside so RemoveAll must unlink it (a populated dir under a
	// read-only parent can't be removed).
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, "f"), []byte("x"), 0o600))
	require.NoError(t, os.Chmod(parent, 0o500))
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	c := &cache{dir: cacheDir}
	err := c.clear(ctx)
	require.Error(t, err)
}
