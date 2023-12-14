package download_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/testh/tu"
)

const (
	urlActorCSV        = "https://sq.io/testdata/actor.csv"
	urlPaymentLargeCSV = "https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv"
	sizeActorCSV       = int64(7641)
)

func TestSlowHeaderServer(t *testing.T) {
	const hello = `Hello World!`
	var srvr *httptest.Server
	serverDelay := time.Second * 200
	srvr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			t.Log("Server request context done")
			return
		case <-time.After(serverDelay):
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", strconv.Itoa(len(hello)))
		_, err := w.Write([]byte(hello))
		assert.NoError(t, err)
	}))
	t.Cleanup(srvr.Close)

	clientHeaderTimeout := time.Second * 2
	c := httpz.NewClient(httpz.OptHeaderTimeout(clientHeaderTimeout))
	req, err := http.NewRequest(http.MethodGet, srvr.URL, nil)
	require.NoError(t, err)
	resp, err := c.Do(req)
	require.Error(t, err)
	require.Nil(t, resp)
	t.Log(err)
}

func TestDownload_redirect(t *testing.T) {
	const hello = `Hello World!`
	serveBody := hello
	lastModified := time.Now().UTC()
	// cacheDir := t.TempDir()
	// FIXME: switch back to temp dir
	cacheDir := filepath.Join("testdata", "download", tu.Name(t.Name()))

	log := slogt.New(t)
	var srvr *httptest.Server
	srvr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := log.With("origin", "server")
		log.Info("Request on /actual", "req", httpz.RequestLogValue(r))
		switch r.URL.Path {
		case "/redirect":
			loc := srvr.URL + "/actual"
			log.Info("Redirecting to", lga.Loc, loc)
			http.Redirect(w, r, loc, http.StatusFound)
		case "/actual":
			if ifm := r.Header.Get("If-Modified-Since"); ifm != "" {
				tm, err := time.Parse(http.TimeFormat, ifm)
				if err != nil {
					log.Error("Failed to parse If-Modified-Since", lga.Err, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				ifModifiedSinceUnix := tm.Unix()
				lastModifiedUnix := lastModified.Unix()

				if lastModifiedUnix <= ifModifiedSinceUnix {
					log.Info("Serving http.StatusNotModified")
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			log.Info("Serving actual: writing bytes")
			b := []byte(serveBody)
			w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
			_, err := w.Write(b)
			assert.NoError(t, err)
		default:
			log.Info("Serving http.StatusNotFound")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srvr.Close)

	ctx := lg.NewContext(context.Background(), log.With("origin", "downloader"))
	loc := srvr.URL + "/redirect"

	dl, err := download.New(t.Name(), httpz.NewDefaultClient(), loc, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	h := download.NewSinkHandler(log.With("origin", "handler"))

	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	gotBody := h.UncachedBufs[0].String()
	require.Equal(t, hello, gotBody)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.UncachedBufs)
	gotFile := h.CachedFiles[0]
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.UncachedBufs)
	gotFile = h.CachedFiles[0]
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)
}

//tr := httpcache.NewTransport(diskcache.New(cacheDir))
//req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
//require.NoError(t, err)
//
//resp, err := tr.RoundTrip(req)
//require.NoError(t, err)
//require.Equal(t, http.StatusOK, resp.StatusCode)
//b, err := io.ReadAll(resp.Body)
//require.NoError(t, err)
//require.Equal(t, serveBody, string(b))
//t.Logf("b: \n\n%s\n\n", b)
//
//resp2, err := tr.RoundTrip(req)
//require.NoError(t, err)
//require.Equal(t, http.StatusOK, resp2.StatusCode)
//
//b, err = io.ReadAll(resp.Body)
//require.NoError(t, err)
//require.Equal(t, serveBody, string(b))
//t.Logf("b: \n\n%s\n\n", b)

//
//ctx := lg.NewContext(context.Background(), log.With("origin", "downloader"))
//loc := srvr.URL + "/redirect"
//loc := srvr.URL + "/actual"

func TestDownload_New(t *testing.T) {
	log := slogt.New(t)
	ctx := lg.NewContext(context.Background(), log)
	const dlURL = urlActorCSV

	// FIXME: switch to temp dir
	cacheDir, err := filepath.Abs(filepath.Join("testdata", "download", tu.Name(t.Name())))
	require.NoError(t, err)
	t.Logf("cacheDir: %s", cacheDir)

	dl, err := download.New(t.Name(), httpz.NewDefaultClient(), dlURL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, download.Uncached, dl.State(ctx))
	sum, ok := dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	h := download.NewSinkHandler(log.With("origin", "handler"))
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.WriteErrors)
	require.Empty(t, h.CachedFiles)
	require.Equal(t, sizeActorCSV, int64(h.UncachedBufs[0].Len()))
	require.Equal(t, download.Fresh, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.WriteErrors)
	require.Empty(t, h.UncachedBufs)
	require.NotEmpty(t, h.CachedFiles)
	gotFileBytes, err := os.ReadFile(h.CachedFiles[0])
	require.NoError(t, err)
	require.Equal(t, sizeActorCSV, int64(len(gotFileBytes)))
	require.Equal(t, download.Fresh, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)

	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, download.Uncached, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.WriteErrors)
}
