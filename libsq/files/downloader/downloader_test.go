package downloader_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/files/downloader"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

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
	const dlURL = sakila.ActorCSVURL

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
	require.Equal(t, int64(sakila.ActorCSVSize), int64(h.Streams[0].Size()))
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
	require.Equal(t, sakila.ActorCSVSize, len(gotFileBytes))
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

func TestCachePreservedOnFailedRefresh(t *testing.T) {
	o := options.Options{files.OptHTTPResponseTimeout.Key(): "10m"}
	ctx := options.NewContext(context.Background(), o)

	var (
		log                 = lgt.New(t)
		srvr                *httptest.Server
		srvrShouldBodyError bool
		srvrShouldNoCache   bool
		srvrMaxAge          = -1 // seconds
		sentBody            string
	)

	srvr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if srvrShouldBodyError {
			// We want the error to happen while reading the body,
			// not send non-200 status code.
			w.Header().Set("Content-Length", "1000")
			sentBody = stringz.UniqSuffix("baaaaad")
			_, err := w.Write([]byte(sentBody))
			assert.NoError(t, err)
			w.(http.Flusher).Flush()
			time.Sleep(time.Millisecond * 10)
			srvr.CloseClientConnections()
			// The client should get an io.ErrUnexpectedEOF.
			return
		}

		// Use "no-cache" to force downloader.getFreshness to return Stale:
		// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control#response_directives

		if srvrShouldNoCache {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Cache-Control", "max-age=0")
		}
		if srvrMaxAge >= 0 {
			w.Header().Set("Cache-Control", "max-age="+strconv.Itoa(srvrMaxAge))
		}

		w.Header().Set("Content-Type", "text/plain")
		sentBody = stringz.UniqSuffix("hello") // Send a unique body each time.
		w.Header().Set("Content-Length", strconv.Itoa(len(sentBody)))
		_, err := w.Write([]byte(sentBody))
		assert.NoError(t, err)
	}))
	t.Cleanup(srvr.Close)

	ctx = lg.NewContext(ctx, log.With("origin", "downloader"))
	cacheDir := filepath.Join(t.TempDir(), stringz.UniqSuffix("dlcache"))
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	h := downloader.NewSinkHandler(log.With("origin", "handler"))

	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.NotEmpty(t, h.Streams)

	stream := h.Streams[0]
	start := time.Now()
	t.Logf("Waiting for download to complete")
	<-stream.Filled()
	t.Logf("Download completed after %s", time.Since(start))
	require.True(t, errors.Is(stream.Err(), io.EOF))
	gotSize, err := dl.Filesize(ctx)
	require.NoError(t, err)
	require.Equal(t, len(sentBody), int(gotSize))

	require.Equal(t, len(sentBody), stream.Size())
	gotFilesize, err := dl.Filesize(ctx)
	require.NoError(t, err)
	require.Equal(t, len(sentBody), int(gotFilesize))

	fpBody, err := dl.CacheFile(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, fpBody)
	fpHeader := filepath.Join(filepath.Dir(fpBody), "header")
	fpChecksums := filepath.Join(filepath.Dir(fpBody), "checksums.txt")
	t.Logf("Cache files:\n- body:       %s\n- header:     %s\n- checksums:  %s",
		fpBody, fpHeader, fpChecksums)

	fiBody1 := tu.MustStat(t, fpBody)
	fiHeader1 := tu.MustStat(t, fpHeader)
	fiChecksums1 := tu.MustStat(t, fpChecksums)

	srvrShouldBodyError = true
	h.Reset()

	// Sleep to allow file modification timestamps to tick
	time.Sleep(time.Millisecond * 10)
	dl.Get(ctx, h.Handler)
	require.Empty(t, h.Errors)
	require.NotEmpty(t, h.Streams)
	stream = h.Streams[0]
	<-stream.Filled()
	err = stream.Err()
	t.Logf("got stream err: %v", err)
	require.Error(t, err)
	require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
	require.Equal(t, len(sentBody), stream.Size())

	// Verify that the server hasn't updated the cache,
	// by checking that the file modification timestamps
	// haven't changed.
	fiBody2 := tu.MustStat(t, fpBody)
	fiHeader2 := tu.MustStat(t, fpHeader)
	fiChecksums2 := tu.MustStat(t, fpChecksums)

	require.True(t, ioz.FileInfoEqual(fiBody1, fiBody2))
	require.True(t, ioz.FileInfoEqual(fiHeader1, fiHeader2))
	require.True(t, ioz.FileInfoEqual(fiChecksums1, fiChecksums2))

	h.Reset()
}
