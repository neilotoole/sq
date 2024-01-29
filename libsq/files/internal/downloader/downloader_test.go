package downloader_test

import (
	"context"
	"errors"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDownloader(t *testing.T) {
	const dlURL = sakila.ActorCSVURL
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	cacheDir := t.TempDir()

	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), dlURL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))
	require.Equal(t, downloader.Uncached, dl.State(ctx))
	sum, ok := dl.Checksum(ctx)
	require.False(t, ok)
	require.Empty(t, sum)

	// Here's our first download, it's not cached, so we should
	// get a stream.
	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Empty(t, gotFile)
	require.NotNil(t, gotStream)
	require.Equal(t, downloader.Uncached, dl.State(ctx))

	// The stream should not be filled yet, because we
	// haven't read from it.
	tu.RequireNoTake(t, gotStream.Filled())

	// And there should be no cache file, because the cache file
	// isn't written until the stream is drained.
	gotFile, gotErr = dl.CacheFile(ctx)
	require.Error(t, gotErr)
	require.Empty(t, gotFile)

	r := gotStream.NewReader(ctx)
	gotStream.Seal()

	// Now we drain the stream, and the cache should magically fill.
	gotN, err := ioz.DrainN(r, true)
	require.NoError(t, err)
	require.Equal(t, sakila.ActorCSVSize, gotN)
	tu.RequireTake(t, gotStream.Filled())
	tu.RequireTake(t, gotStream.Done())
	require.Equal(t, sakila.ActorCSVSize, gotStream.Size())
	require.Equal(t, downloader.Fresh, dl.State(ctx))

	// Now we should be able to access the cache.
	sum, ok = dl.Checksum(ctx)
	require.True(t, ok)
	require.NotEmpty(t, sum)
	gotFile, gotErr = dl.CacheFile(ctx)
	require.NoError(t, gotErr)
	require.NotEmpty(t, gotFile)
	gotSize, gotErr := ioz.Filesize(gotFile)
	require.NoError(t, err)
	require.Equal(t, sakila.ActorCSVSize, int(gotSize))

	// Let's download again, and verify that the cache is used.
	gotFile, gotStream, gotErr = dl.Get(ctx)
	require.Nil(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)

	gotFileBytes, err := os.ReadFile(gotFile)
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
}

func TestDownloader_redirect(t *testing.T) {
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

	gotFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.NotNil(t, gotStream)
	require.Empty(t, gotFile)
	gotBody := tu.ReadToString(t, gotStream.NewReader(ctx))
	require.Equal(t, hello, gotBody)

	gotFile, gotStream, gotErr = dl.Get(ctx)
	require.NoError(t, gotErr)
	require.NotEmpty(t, gotFile)
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)
}

func TestCachePreservedOnFailedRefresh(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	var (
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

	cacheDir := filepath.Join(t.TempDir(), stringz.UniqSuffix("dlcache"))
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	gotGetFile, gotStream, gotErr := dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Empty(t, gotGetFile)
	require.NotNil(t, gotStream)
	tu.RequireNoTake(t, gotStream.Filled())
	r := gotStream.NewReader(ctx)
	gotStream.Seal()
	t.Logf("Waiting for download to complete")
	start := time.Now()

	gotN, gotErr := ioz.DrainN(r, true)
	t.Logf("Download completed after %s", time.Since(start))
	tu.RequireTake(t, gotStream.Filled())
	tu.RequireTake(t, gotStream.Done())
	require.Equal(t, len(sentBody), gotN)

	require.True(t, errors.Is(gotStream.Err(), io.EOF))
	gotSize, err := dl.Filesize(ctx)
	require.NoError(t, err)
	require.Equal(t, len(sentBody), int(gotSize))
	require.Equal(t, len(sentBody), gotStream.Size())
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

	// Sleep to allow file modification timestamps to tick
	time.Sleep(time.Millisecond * 10)
	gotGetFile, gotStream, gotErr = dl.Get(ctx)
	require.NoError(t, gotErr)
	require.Empty(t, gotGetFile,
		"gotFile should be empty, because the server returned an error during cache write")
	require.NotNil(t, gotStream,
		"gotStream should not be empty, because the download was in fact initiated")

	r = gotStream.NewReader(ctx)
	gotStream.Seal()

	gotN, gotErr = ioz.DrainN(r, true)
	require.Equal(t, len(sentBody), gotN)
	tu.RequireTake(t, gotStream.Filled())
	tu.RequireNoTake(t, gotStream.Done())

	gotErr2 := gotStream.Err()
	require.Error(t, gotErr2)
	require.True(t, errors.Is(gotErr, gotErr2))
	t.Logf("got stream err: %v", gotErr)

	require.True(t, errors.Is(gotErr, io.ErrUnexpectedEOF))
	require.Equal(t, len(sentBody), gotStream.Size())

	// Verify that the server hasn't updated the cache,
	// by checking that the file modification timestamps
	// haven't changed.
	fiBody2 := tu.MustStat(t, fpBody)
	fiHeader2 := tu.MustStat(t, fpHeader)
	fiChecksums2 := tu.MustStat(t, fpChecksums)

	require.True(t, ioz.FileInfoEqual(fiBody1, fiBody2))
	require.True(t, ioz.FileInfoEqual(fiHeader1, fiHeader2))
	require.True(t, ioz.FileInfoEqual(fiChecksums1, fiChecksums2))
}
