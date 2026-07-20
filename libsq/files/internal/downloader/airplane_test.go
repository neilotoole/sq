package downloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// TestDownloader_StaleRefreshTransportError is a regression test for a
// nil-pointer dereference panic in Downloader.get. When a stale cache was
// refreshed and the refresh hit a transport error (e.g. the network is
// unavailable), get dereferenced the nil response body before reaching the
// cacheFileOnError path. This is precisely the "airplane mode" scenario that
// OptContinueOnError is designed to handle.
//
// With OptContinueOnError true (the default), the stale cached file must be
// returned. With it false, the transport error must be returned (and not
// panic).
func TestDownloader_StaleRefreshTransportError(t *testing.T) {
	const body = "hello world"

	newStaleServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// max-age=0 makes the cached response immediately stale, so the
			// next Get triggers a conditional refresh request.
			w.Header().Set("Cache-Control", "max-age=0")
			w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write([]byte(body))
		}))
	}

	// populate downloads body once, into a fresh downloader+cache, then kills
	// the server so a subsequent refresh fails at the transport layer.
	populate := func(t *testing.T, ctx context.Context, cacheDir string) *downloader.Downloader {
		t.Helper()
		srvr := newStaleServer()
		dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, cacheDir)
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

		// Kill the server: the next request fails at the transport layer.
		srvr.Close()
		return dl
	}

	t.Run("continue_on_error_true", func(t *testing.T) {
		ctx := lg.NewContext(context.Background(), lgt.New(t))
		ctx = options.NewContext(ctx, options.Options{
			downloader.OptContinueOnError.Key(): true,
		})
		dl := populate(t, ctx, tu.TempDir(t))

		// Airplane mode: the stale cached file is returned, no panic.
		gotFile, gotStream, gotErr := dl.Get(ctx)
		require.NoError(t, gotErr)
		require.Nil(t, gotStream)
		require.NotEmpty(t, gotFile, "expected stale cached file in airplane mode")
		require.Equal(t, body, tu.ReadFileToString(t, gotFile))
	})

	t.Run("continue_on_error_false", func(t *testing.T) {
		ctx := lg.NewContext(context.Background(), lgt.New(t))
		ctx = options.NewContext(ctx, options.Options{
			downloader.OptContinueOnError.Key(): false,
		})
		dl := populate(t, ctx, tu.TempDir(t))

		// With continue-on-error disabled, the transport error is returned
		// (and crucially, no panic).
		gotFile, gotStream, gotErr := dl.Get(ctx)
		require.Error(t, gotErr)
		require.Nil(t, gotStream)
		require.Empty(t, gotFile)
	})
}
