package download_test

import (
	"bufio"
	"bytes"
	"context"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/stretchr/testify/require"
)

const (
	urlPaymentLargeCSV = "https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv"
	urlActorCSV        = "https://sq.io/testdata/actor.csv"
	sizeActorCSV       = int64(7641)
)

func TestDownload_redirect(t *testing.T) {
	const hello = `Hello World!`
	var serveBody = hello
	lastModified := time.Now().UTC()
	//cacheDir := t.TempDir()
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

	dl, err := download.New(httpz.NewDefaultClient(), loc, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	h := newTestHandler(log.With("origin", "handler"))

	dl.Get(ctx, h.Handler)
	require.Empty(t, h.errors)
	gotBody := h.bufs[0].String()
	require.Equal(t, hello, gotBody)

	h.reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.errors)
	require.Empty(t, h.bufs)
	gotFile := h.cacheFiles[0]
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)

	h.reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.errors)
	require.Empty(t, h.bufs)
	gotFile = h.cacheFiles[0]
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

	dl, err := download.New(nil, dlURL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, download.Uncached, dl.State(ctx))
	sum, ok := dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	h := newTestHandler(log.With("origin", "handler"))
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.errors)
	require.Empty(t, h.cacheFiles)
	require.Equal(t, sizeActorCSV, int64(h.bufs[0].Len()))
	require.Equal(t, download.Fresh, dl.State(ctx))
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)

	h.reset()
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.errors)
	require.Empty(t, h.bufs)
	require.NotEmpty(t, h.cacheFiles)
	gotFileBytes, err := os.ReadFile(h.cacheFiles[0])
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
}

type testHandler struct {
	download.Handler
	mu         sync.Mutex
	log        *slog.Logger
	errors     []error
	cacheFiles []string
	bufs       []*bytes.Buffer
	writeErrs  []error
}

func (th *testHandler) reset() {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.errors = nil
	th.cacheFiles = nil
	th.bufs = nil
	th.writeErrs = nil
}

func newTestHandler(log *slog.Logger) *testHandler {
	th := &testHandler{log: log}
	th.Cached = func(fp string) {
		log.Info("Cached", lga.File, fp)
		th.mu.Lock()
		defer th.mu.Unlock()
		th.cacheFiles = append(th.cacheFiles, fp)
	}

	th.Uncached = func() (io.WriteCloser, func(error)) {
		log.Info("Uncached")
		th.mu.Lock()
		defer th.mu.Unlock()
		buf := &bytes.Buffer{}
		th.bufs = append(th.bufs, buf)
		return ioz.WriteCloser(buf), func(err error) {
			th.mu.Lock()
			defer th.mu.Unlock()
			th.writeErrs = append(th.writeErrs, err)
		}
	}

	th.Error = func(err error) {
		log.Info("Error", lga.Err, err)
		th.mu.Lock()
		defer th.mu.Unlock()
		th.errors = append(th.errors, err)
	}
	return th
}

func TestMisc(t *testing.T) {
	// FIXME: delete
	br := bufio.NewReader(strings.NewReader("huzzah"))

	cr := contextio.NewReader(context.TODO(), br)

	t.Logf("cr: %T", cr)

	var wt io.WriterTo
	wt = br
	_ = wt

	var ok bool
	wt, ok = cr.(io.WriterTo)
	require.True(t, ok)
}
