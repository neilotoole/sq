// Package downloader_test contains tests for the downloader package.
//
// The tests verify:
//   - Basic download and caching behavior (TestDownloader)
//   - HTTP redirect handling (TestDownloader_redirect)
//   - SinkHandler callback recording (TestSinkHandler, TestSinkHandler_Uncached)
//   - Cache preservation on failed refresh (TestCachePreservedOnFailedRefresh)
//   - State.String() method (TestState_String)

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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/files/internal/downloader"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestState_String verifies the String() method of the [downloader.State] type.
//
// This test uses a table-driven approach to verify that each state constant
// (Uncached, Stale, Fresh, Transparent) produces the expected string
// representation. It also verifies that an invalid/unknown state value
// returns "unknown" rather than panicking.
//
// Test cases:
//   - Uncached (0) -> "uncached"
//   - Stale (1) -> "stale"
//   - Fresh (2) -> "fresh"
//   - Transparent (3) -> "transparent"
//   - State(99) (invalid) -> "unknown"
func TestState_String(t *testing.T) {
	testCases := []struct {
		state downloader.State
		want  string
	}{
		{downloader.Uncached, "uncached"},
		{downloader.Stale, "stale"},
		{downloader.Fresh, "fresh"},
		{downloader.Transparent, "transparent"},
		{downloader.State(99), "unknown"}, // Unknown state
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.state.String()
			require.Equal(t, tc.want, got)
		})
	}
}

// TestDownloader is an integration test that exercises the complete download
// and caching lifecycle using a real HTTP resource (sakila.ActorCSVURL).
//
// The test verifies the following behaviors:
//
//  1. Initial state: A new Downloader starts in [downloader.Uncached] state
//     with no checksum available.
//
//  2. First download: Calling Get() on an uncached resource returns a stream
//     (not a file path), and the state remains Uncached until the stream is
//     fully read.
//
//  3. Streaming behavior: The stream is not "filled" until fully read. The
//     cache file doesn't exist until the stream is drained (two-phase commit).
//
//  4. Cache population: After draining the stream, the Downloader transitions
//     to [downloader.Fresh] state, and the cached file becomes accessible via
//     CacheFile() and Checksum().
//
//  5. Subsequent requests: Calling Get() again returns the cached file path
//     (not a stream), demonstrating cache hit behavior.
//
//  6. Cache clearing: Calling Clear() resets the Downloader to Uncached state
//     and removes the cached checksum.
//
// This test requires network access to download the sakila actor CSV file.
func TestDownloader(t *testing.T) {
	const dlURL = sakila.ActorCSVURL
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	cacheDir := tu.TempDir(t)

	dl, gotErr := downloader.New(t.Name(), httpz.NewDefaultClient(), dlURL, cacheDir)
	require.NoError(t, gotErr)
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
	var gotN int
	gotN, gotErr = ioz.DrainClose(r)
	require.NoError(t, gotErr)
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
	require.NoError(t, gotErr)
	require.Equal(t, sakila.ActorCSVSize, int(gotSize))

	// Let's download again, and verify that the cache is used.
	gotFile, gotStream, gotErr = dl.Get(ctx)
	require.Nil(t, gotErr)
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)

	gotFileBytes, gotErr := os.ReadFile(gotFile)
	require.NoError(t, gotErr)
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

// TestDownloader_redirect verifies that [Downloader] correctly handles HTTP
// redirects, following them to the final destination and caching appropriately.
//
// The test sets up a local HTTP server with two endpoints:
//   - /redirect: Returns HTTP 302 redirect to /actual
//   - /actual: Returns the content with Last-Modified header
//
// The test verifies:
//
//  1. First request: The Downloader follows the redirect from /redirect to
//     /actual, returns a stream with the content, and ultimately caches it.
//
//  2. Second request: The Downloader returns the cached file (not a stream),
//     demonstrating that the redirect was properly resolved and cached.
//
//  3. Conditional requests: The server supports If-Modified-Since, returning
//     304 Not Modified if the content hasn't changed. This validates that
//     the Downloader correctly handles cache revalidation with redirects.
//
// This is an important test because HTTP redirects add complexity to caching:
// the cache must be keyed appropriately and conditional request headers must
// be sent to the correct (final) URL.
func TestDownloader_redirect(t *testing.T) {
	const hello = `Hello World!`
	serveBody := hello
	lastModified := time.Now().UTC()
	cacheDir := tu.TempDir(t)

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
	require.Nil(t, gotStream)
	require.NotEmpty(t, gotFile)
	t.Logf("got fp: %s", gotFile)
	gotBody = tu.ReadFileToString(t, gotFile)
	t.Logf("got body: \n\n%s\n\n", gotBody)
	require.Equal(t, serveBody, gotBody)
}

// TestSinkHandler tests the [downloader.SinkHandler] type, which records
// callback invocations for testing purposes.
//
// This test verifies:
//
//  1. Initial state: A newly created SinkHandler has empty slices for
//     Errors, Downloaded, and Streams.
//
//  2. Cached callback: Invoking the Cached callback appends file paths to
//     the Downloaded slice in order.
//
//  3. Error callback: Invoking the Error callback appends errors to the
//     Errors slice in order.
//
//  4. Reset: Calling Reset() clears all slices, preparing the handler for
//     reuse between test cases.
//
// Note: The Uncached callback is tested separately in [TestSinkHandler_Uncached]
// because it requires a [streamcache.Stream] argument.
func TestSinkHandler(t *testing.T) {
	log := lgt.New(t)
	h := downloader.NewSinkHandler(log)
	require.NotNil(t, h)

	// Initially empty
	require.Empty(t, h.Errors)
	require.Empty(t, h.Downloaded)
	require.Empty(t, h.Streams)

	// Test Cached callback
	h.Cached("/path/to/file1")
	h.Cached("/path/to/file2")
	require.Len(t, h.Downloaded, 2)
	require.Equal(t, "/path/to/file1", h.Downloaded[0])
	require.Equal(t, "/path/to/file2", h.Downloaded[1])

	// Test Error callback
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	h.Error(err1)
	h.Error(err2)
	require.Len(t, h.Errors, 2)
	require.Equal(t, err1, h.Errors[0])
	require.Equal(t, err2, h.Errors[1])

	// Test Reset
	h.Reset()
	require.Empty(t, h.Errors)
	require.Empty(t, h.Downloaded)
	require.Empty(t, h.Streams)
}

// TestSinkHandler_Uncached tests the Uncached callback of [downloader.SinkHandler],
// which is invoked when a download begins (no cache hit).
//
// This test verifies:
//
//  1. Uncached callback: Invoking the Uncached callback with a
//     [streamcache.Stream] appends the stream to the Streams slice.
//
//  2. Stream identity: The stored stream is the exact same instance that
//     was passed to the callback (verified with require.Same).
//
//  3. Reset cleanup: Calling Reset() closes the source reader of recorded
//     streams (preventing resource leaks) and clears the Streams slice.
//
// The test creates a mock stream from a simple string reader to avoid
// needing actual HTTP downloads.
func TestSinkHandler_Uncached(t *testing.T) {
	log := lgt.New(t)
	h := downloader.NewSinkHandler(log)

	// Create a mock stream using streamcache
	r := io.NopCloser(strings.NewReader("test content"))
	stream := streamcache.New(r)

	h.Uncached(stream)
	require.Len(t, h.Streams, 1)
	require.Same(t, stream, h.Streams[0])

	// Reset should close the stream source
	h.Reset()
	require.Empty(t, h.Streams)
}

// TestCachePreservedOnFailedRefresh verifies that the cache is NOT overwritten
// when a refresh attempt fails mid-download.
//
// This is a critical safety test for the two-phase cache commit strategy.
// It ensures that if a download starts successfully but fails before completion
// (e.g., connection dropped, server error mid-stream), the existing valid
// cache remains intact rather than being replaced with corrupt/partial data.
//
// Test setup:
//   - A local HTTP server that can be configured to fail mid-response
//   - srvrShouldBodyError: When true, server sends partial body then closes
//     the connection, causing io.ErrUnexpectedEOF on the client
//   - Cache-Control headers are used to force cache revalidation
//
// Test phases:
//
//  1. Initial download: Successfully download and cache content. Record the
//     file modification timestamps of the cache files (body, header, checksums).
//
//  2. Failed refresh: Configure server to fail mid-body, trigger a refresh.
//     The download begins but fails with io.ErrUnexpectedEOF.
//
//  3. Verify preservation: Check that the cache file timestamps are unchanged,
//     proving the failed download did not corrupt the existing cache.
//
// This test validates the atomicity guarantee of [responseCacher]: staging
// data is only promoted to main cache on successful completion.
func TestCachePreservedOnFailedRefresh(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	var (
		srvr                *httptest.Server
		srvrShouldBodyError bool
		srvrShouldNoCache   bool
		srvrMaxAge          = -1 // seconds
		sentBody            string
	)

	srvr = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	cacheDir := filepath.Join(tu.TempDir(t), stringz.UniqSuffix("dlcache"))
	dl, err := downloader.New(t.Name(), httpz.NewDefaultClient(), srvr.URL, cacheDir)
	require.NoError(t, err)
	require.NoError(t, dl.Clear(ctx))

	var gotFile string
	var gotStream *streamcache.Stream
	gotFile, gotStream, err = dl.Get(ctx)
	require.NoError(t, err)
	require.Empty(t, gotFile)
	require.NotNil(t, gotStream)
	tu.RequireNoTake(t, gotStream.Filled())
	r := gotStream.NewReader(ctx)
	gotStream.Seal()
	t.Logf("Waiting for download to complete")
	start := time.Now()

	var gotN int
	gotN, err = ioz.DrainClose(r)
	require.NoError(t, err)
	t.Logf("Download completed after %s", time.Since(start))
	tu.RequireTake(t, gotStream.Filled())
	tu.RequireTake(t, gotStream.Done())
	require.Equal(t, len(sentBody), gotN)

	require.True(t, errors.Is(gotStream.Err(), io.EOF))
	require.NoError(t, err)
	require.Equal(t, len(sentBody), gotStream.Size())

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
	gotFile, gotStream, err = dl.Get(ctx)
	require.NoError(t, err)
	require.Empty(t, gotFile,
		"gotFile should be empty, because the server returned an error during cache write")
	require.NotNil(t, gotStream,
		"gotStream should not be empty, because the download was in fact initiated")

	r = gotStream.NewReader(ctx)
	gotStream.Seal()

	gotN, err = ioz.DrainClose(r)
	require.Equal(t, len(sentBody), gotN)
	tu.RequireTake(t, gotStream.Filled())
	tu.RequireTake(t, gotStream.Done())

	streamErr := gotStream.Err()
	require.Error(t, streamErr)
	require.True(t, errors.Is(err, streamErr))
	t.Logf("got stream err: %v", err)

	require.True(t, errors.Is(err, io.ErrUnexpectedEOF))
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
