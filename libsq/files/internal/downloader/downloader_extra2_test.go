// This file contains additional black-box tests for the Downloader public API,
// targeting branches in Clear and get that the existing tests don't reach,
// using httptest servers only (no external network).

package downloader_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/testh/tu"
)

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
