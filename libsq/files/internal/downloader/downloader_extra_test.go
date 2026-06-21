// This file contains additional black-box tests for the Downloader public API,
// targeting branches in Get/get, New, CacheFile, Checksum, and Clear that the
// existing tests don't reach. Tests use httptest servers; no external network.

package downloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/testh/tu"
)

// newDownloaderTestCtx returns a context carrying a test logger.
func newDownloaderTestCtx(t *testing.T) context.Context {
	t.Helper()
	return lg.NewContext(context.Background(), lgt.New(t))
}

// TestNew_invalidURL verifies that New rejects a syntactically invalid URL via
// url.ParseRequestURI.
func TestNew_invalidURL(t *testing.T) {
	_, err := downloader.New(t.Name(), httpz.NewDefaultClient(), "://not-a-url", tu.TempDir(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid download URL")
}

// TestCacheFile_noCacheConfigured verifies that CacheFile on a brand-new
// downloader (Get never called, so dl.cache is nil) returns the
// "cache doesn't exist" error.
func TestCacheFile_noCacheConfigured(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), "http://example.com", tu.TempDir(t))
	require.NoError(t, err)

	fp, err := dl.CacheFile(ctx)
	require.Error(t, err)
	require.Empty(t, fp)
	require.Contains(t, err.Error(), "cache doesn't exist")
}

// TestCacheFile_cacheEmpty verifies that CacheFile returns the "no cache for"
// error when caching is enabled (dl.cache set after a Get with OptCache) but
// nothing has actually been cached yet, e.g. because the response was
// no-store.
func TestCacheFile_cacheEmpty(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("data"))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)

	// Get sets dl.cache (OptCache defaults true) but the no-store response is
	// not stored, so the cache stays empty.
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)

	fp, err := dl.CacheFile(ctx)
	require.Error(t, err)
	require.Empty(t, fp)
	require.Contains(t, err.Error(), "no cache for")
}

// TestChecksum_noCacheConfigured verifies Checksum returns ok=false on a
// brand-new downloader (dl.cache nil).
func TestChecksum_noCacheConfigured(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), "http://example.com", tu.TempDir(t))
	require.NoError(t, err)

	sum, ok := dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)
}

// TestClear_noCacheConfigured verifies Clear returns nil on a brand-new
// downloader (dl.cache nil).
func TestClear_noCacheConfigured(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), "http://example.com", tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
}

// TestGet_cacheDisabled verifies that with OptCache=false, Get never sets up a
// disk cache: dl.cache stays nil, every Get returns a fresh stream, and the
// downloader reports Uncached state. This exercises the non-cacheable
// return path at the end of get().
func TestGet_cacheDisabled(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	ctx = options.NewContext(ctx, options.Options{
		downloader.OptCache.Key(): false,
	})

	const body = "no-cache body"
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)

	require.Equal(t, downloader.Uncached, dl.State(ctx))

	for range 2 {
		gotFile, gotStream, gotErr := dl.Get(ctx)
		require.NoError(t, gotErr)
		require.Empty(t, gotFile, "cache disabled: should always stream, never return a file")
		require.NotNil(t, gotStream)
		r := gotStream.NewReader(ctx)
		gotStream.Seal()
		require.Equal(t, body, tu.ReadToString(t, r))
	}

	// With caching disabled, CacheFile reports no cache configured.
	_, err = dl.CacheFile(ctx)
	require.Error(t, err)
}

// TestGet_notModifiedRefresh exercises the 304 Not Modified refresh path: the
// server first serves a body with Last-Modified and max-age=0 (immediately
// stale), then on the conditional refresh (If-Modified-Since) returns 304. The
// second Get must return the cached file, driving the resp==cachedResp +
// writeHeader branch in get().
func TestGet_notModifiedRefresh(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "conditional body"
	lastModified := time.Now().UTC().Add(-time.Hour)
	var hits int32

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("If-Modified-Since") != "" {
			// Refresh request: nothing changed.
			w.WriteHeader(http.StatusNotModified)
			return
		}
		// First request: serve body, marked immediately stale so the next Get
		// triggers a conditional refresh.
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		w.Header().Set("Cache-Control", "max-age=0")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	// First Get: populate the cache.
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)

	// Cached response is stale (max-age=0).
	require.Equal(t, downloader.Stale, dl.State(ctx))

	// Second Get: conditional refresh returns 304 -> cached file returned.
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)
	require.Equal(t, body, tu.ReadFileToString(t, gotFile))
	require.GreaterOrEqual(t, atomic.LoadInt32(&hits), int32(2))
}

// TestGet_staleIfErrorServesStaleOn500 exercises the
// resp.StatusCode >= 500 && canStaleOnError branch: a stale cache exists whose
// stored response carries stale-if-error, and the refresh returns HTTP 500. The
// stale cached file must be returned.
func TestGet_staleIfErrorServesStaleOn500(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "stale-if-error body"
	var fail atomic.Bool

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Immediately stale, but allows stale-on-error for a long window.
		w.Header().Set("Cache-Control", "max-age=0, stale-if-error=86400")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	// Populate cache.
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Stale, dl.State(ctx))

	// Now make the server return 500 on refresh; stale-if-error serves cache.
	fail.Store(true)
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)
	require.Equal(t, body, tu.ReadFileToString(t, gotFile))
}

// TestGet_continueOnErrorServesStaleOn500 exercises the path where the refresh
// returns a non-200 status with NO stale-if-error directive, but
// OptContinueOnError is true: cacheFileOnError returns the stale file.
func TestGet_continueOnErrorServesStaleOn500(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	ctx = options.NewContext(ctx, options.Options{
		downloader.OptContinueOnError.Key(): true,
	})

	const body = "continue-on-error body"
	var fail atomic.Bool

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Stale immediately, but no stale-if-error directive.
		w.Header().Set("Cache-Control", "max-age=0")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Stale, dl.State(ctx))

	fail.Store(true)
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)
	require.Equal(t, body, tu.ReadFileToString(t, gotFile))
}

// TestGet_500NoCacheStreams documents the behavior when the server returns a
// non-200 status and there is NO prior cache: because the 500 response carries
// no no-store directive, get() takes the no-cachedResp "else" branch (which
// only calls do()), then proceeds to stream/cache the (empty) body. The
// unexpected-status handling only applies when a cachedResp is present, so a
// first-ever 500 is streamed rather than surfaced as an error.
func TestGet_500NoCacheStreams(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Empty(t, gotFile)
	require.NotNil(t, gotStream)
	r := gotStream.NewReader(ctx)
	gotStream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
}

// TestGet_noStoreNotCached verifies that a no-store response is streamed but
// never cached: each Get returns a fresh stream and the state stays Uncached.
func TestGet_noStoreNotCached(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "no-store body"
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	for range 2 {
		gotFile, gotStream, gotErr := dl.Get(ctx)
		require.NoError(t, gotErr)
		require.Empty(t, gotFile)
		require.NotNil(t, gotStream)
		r := gotStream.NewReader(ctx)
		gotStream.Seal()
		require.Equal(t, body, tu.ReadToString(t, r))
	}
	require.Equal(t, downloader.Uncached, dl.State(ctx))
}

// TestGet_etagRevalidation exercises the ETag conditional-request branch in
// get(): the cached response carries an ETag, so the stale refresh sends an
// If-None-Match header and the server replies 304, returning the cached file.
func TestGet_etagRevalidation(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "etag body"
	const etag = `"v1-etag"`

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "max-age=0")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Stale, dl.State(ctx))

	// Refresh: If-None-Match -> 304 -> cached file returned.
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)
	require.Equal(t, body, tu.ReadFileToString(t, gotFile))
}

// TestGet_varyEcho exercises the Vary-handling loop in get(): when the response
// declares a Vary header, get() iterates the varied keys to record request
// values under "X-Varied-" keys. The downloader's internally-built request
// carries no user headers, so the reqValue != "" store is not taken, but the
// loop body still runs.
func TestGet_varyEcho(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "vary body"
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Set("Cache-Control", "max-age=300")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	// The default HTTP client sets Accept-Encoding: gzip, which matches the
	// Vary header and drives the reqValue != "" echo branch.
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
}

// TestGet_staleRefreshNewBody exercises the branch where a stale cache exists
// and the refresh returns a fresh 200 with a *different* body (not 304). The
// old cached response is closed (the cachedResp != nil else-branch in get) and
// a new responseCacher streams and caches the new body.
func TestGet_staleRefreshNewBody(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	var second atomic.Bool
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body := "first body"
		if second.Load() {
			body = "second body, different"
		}
		// max-age=0 makes the cached response immediately stale, forcing a
		// refresh on the next Get. No Last-Modified/ETag, so the refresh is a
		// plain GET returning a fresh 200 (not 304).
		w.Header().Set("Cache-Control", "max-age=0")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	// First Get: cache "first body".
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Stale, dl.State(ctx))

	// Second Get: stale cache, refresh returns a fresh 200 with a new body.
	second.Store(true)
	_, stream2, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream2, "fresh non-304 refresh should return a new stream")
	r2 := stream2.NewReader(ctx)
	stream2.Seal()
	require.Equal(t, "second body, different", tu.ReadToString(t, r2))

	// The new body is now cached.
	fp, err := dl.CacheFile(ctx)
	require.NoError(t, err)
	require.Equal(t, "second body, different", tu.ReadFileToString(t, fp))
}

// TestDo_requestTimeout exercises the do() deadline path: a request timeout
// fires against a server that sleeps, and Get returns an error (no cache to
// fall back on). The url.Error/DeadlineExceeded unwrap branch trims the
// "GET <url>" prefix.
func TestDo_requestTimeout(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("too late"))
	}))
	t.Cleanup(srvr.Close)

	client := httpz.NewClient(httpz.DefaultUserAgent, httpz.OptRequestTimeout(time.Millisecond))
	dl, err := downloader.New(t.Name(), client, srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.Error(t, gotErr)
	require.Empty(t, gotFile)
	require.Nil(t, gotStream)
}

// TestClear_afterGet covers the dl.cache != nil branch of Clear: a successful
// Get sets dl.cache, after which Clear delegates to cache.clear and removes the
// cached files.
func TestClear_afterGet(t *testing.T) {
	ctx := newDownloaderTestCtx(t)

	const body = "clear me"
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "max-age=300")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)

	// Get populates the cache (and sets dl.cache).
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Fresh, dl.State(ctx))

	// Clear must hit the dl.cache != nil branch and wipe the cache.
	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, downloader.Uncached, dl.State(ctx))
	_, err = dl.CacheFile(ctx)
	require.Error(t, err)
}

// TestGet_500NoContinueReturnsError covers the get() default-case error return
// (the resp.StatusCode != http.StatusOK path that yields "", nil, err): a stale
// cache exists, the refresh returns HTTP 500 with no stale-if-error directive,
// and OptContinueOnError is false, so no cached file is served and the
// unexpected-status error is returned.
func TestGet_500NoContinueReturnsError(t *testing.T) {
	ctx := newDownloaderTestCtx(t)
	ctx = options.NewContext(ctx, options.Options{
		downloader.OptContinueOnError.Key(): false,
	})

	const body = "stale body"
	var fail atomic.Bool
	srvr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Immediately stale, no stale-if-error -> canStaleOnError is false.
		w.Header().Set("Cache-Control", "max-age=0")
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srvr.Close)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, tu.TempDir(t))
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	// Populate the cache.
	_, stream, err := dl.Get(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
	r := stream.NewReader(ctx)
	stream.Seal()
	_, err = ioz.DrainClose(r)
	require.NoError(t, err)
	require.Equal(t, downloader.Stale, dl.State(ctx))

	// Refresh returns 500; with continue-on-error false and no stale-if-error,
	// get() returns the unexpected-status error and no file/stream.
	fail.Store(true)
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.Error(t, gotErr)
	require.Empty(t, gotFile)
	require.Nil(t, gotStream)
	require.Contains(t, gotErr.Error(), "unexpected HTTP status")
}
