package downloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/downloader"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
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
	c := httpz.NewClient(httpz.OptRequestTimeout(clientHeaderTimeout))
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
	cacheDir := t.TempDir()

	log := lgt.New(t)
	var srvr *httptest.Server
	srvr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srvrLog := log.With("origin", "server")
		srvrLog.Info("Request on /actual", "req", httpz.RequestLogValue(r))
		switch r.URL.Path {
		case "/redirect":
			loc := srvr.URL + "/actual"
			srvrLog.Info("Redirecting to", lga.Loc, loc)
			http.Redirect(w, r, loc, http.StatusFound)
		case "/actual":
			if ifm := r.Header.Get("If-Modified-Since"); ifm != "" {
				tm, err := time.Parse(http.TimeFormat, ifm)
				if err != nil {
					srvrLog.Error("Failed to parse If-Modified-Since", lga.Err, err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				ifModifiedSinceUnix := tm.Unix()
				lastModifiedUnix := lastModified.Unix()

				if lastModifiedUnix <= ifModifiedSinceUnix {
					srvrLog.Info("Serving http.StatusNotModified")
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}

			srvrLog.Info("Serving actual: writing bytes")
			b := []byte(serveBody)
			w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
			_, err := w.Write(b)
			assert.NoError(t, err)
		default:
			srvrLog.Info("Serving http.StatusNotFound")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srvr.Close)

	ctx := lg.NewContext(context.Background(), log.With("origin", "downloader"))
	loc := srvr.URL + "/redirect"

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), loc, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	h := downloader.NewSinkHandler(log.With("origin", "handler"))

	tu.MustAbsFilepath()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	gotBody := tu.ReadToString(t, h.Streams[0].NewReader(ctx))
	require.Equal(t, hello, gotBody)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.Streams)
	gotFile := h.Downloaded[0]
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.Streams)
	gotFile = h.Downloaded[0]
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)
}

func TestDownload_New(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)
	const dlURL = urlActorCSV

	cacheDir := t.TempDir()
	t.Logf("cacheDir: %s", cacheDir)

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), dlURL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, downloader.Uncached, dl.State(ctx))
	sum, ok := dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	h := downloader.NewSinkHandler(log.With("origin", "handler"))
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.Downloaded)
	require.Equal(t, 1, len(h.Streams))
	require.Equal(t, sizeActorCSV, int64(h.Streams[0].Size()))
	require.Equal(t, downloader.Fresh, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.Empty(t, h.Streams)
	require.NotEmpty(t, h.Downloaded)
	gotFileBytes, err := os.ReadFile(h.Downloaded[0])
	require.NoError(t, err)
	require.Equal(t, sizeActorCSV, int64(len(gotFileBytes)))
	require.Equal(t, downloader.Fresh, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)

	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, downloader.Uncached, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	h.Reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
}
