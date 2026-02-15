// Package downloader provides a mechanism for downloading files from HTTP/S
// URLs, with support for RFC-compliant HTTP caching.
//
// # Overview
//
// The package implements a disk-based cache for HTTP downloads, supporting
// standard HTTP cache-control semantics including freshness validation,
// conditional requests (If-Modified-Since, If-None-Match), and the
// stale-if-error extension (RFC 5861).
//
// # Usage
//
// The entrypoint is [New], which creates a [Downloader] for a specific URL.
// Use [Downloader.Get] to retrieve the file, which returns either:
//   - A filepath to an already-cached file (cache hit)
//   - A [streamcache.Stream] for an in-progress download (cache miss)
//   - An error if the download fails and no cached version is available
//
// # Cache Architecture
//
// The cache uses a two-stage write approach with "staging" and "main"
// directories. New downloads are written to staging first, then atomically
// promoted to main upon successful completion. This prevents partial or
// corrupt downloads from polluting the cache.
//
// Each cached download consists of three files:
//   - header: The HTTP response headers
//   - body: The response body content
//   - checksums.txt: SHA-256 checksum for integrity verification
//
// # Options
//
// The downloader respects two context options:
//   - [OptCache]: Enable/disable caching (default: true)
//   - [OptContinueOnError]: Return stale cache on refresh failure (default: true)
//
// # Acknowledgement
//
// This package is a heavily customized fork of
// https://github.com/gregjones/httpcache, via bitcomplete/download.
package downloader

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neilotoole/streamcache"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
)

// OptContinueOnError controls behavior when a cache refresh fails. When true
// (the default), the downloader returns the stale cached file instead of an
// error. This enables "airplane mode" functionality where sq can continue
// working with cached data when the network is unavailable.
var OptContinueOnError = options.NewBool(
	"download.refresh.ok-on-err",
	nil,
	true,
	"Continue with stale download if refresh fails",
	`Continue with stale download if refresh fails. This option applies if a download
is in the cache, but is considered stale, and a refresh attempt fails. If set to
true, the refresh error is logged, and the stale download is returned. This is a
sort of "Airplane Mode" for downloads: when true, sq continues with the cached
download when the network is unavailable. If false, an error is returned instead.`,
	options.TagSource,
)

// OptCache controls whether downloads are cached to disk. When true (the
// default), downloaded files are stored in the cache directory and reused
// on subsequent requests if still fresh. When false, each request triggers
// a new download and no caching occurs.
var OptCache = options.NewBool(
	"download.cache",
	nil,
	true,
	"Cache downloads",
	`Cache downloaded remote files. When false, the download cache is not used and
the file is re-downloaded on each command.`,
	options.TagSource,
)

// State is an enumeration of caching states based on the cache-control
// values of the request and the response.
//
//   - Uncached indicates the item is not cached.
//   - Fresh indicates that the cached item can be returned.
//   - Stale indicates that the cached item needs validating before it is returned.
//   - Transparent indicates the cached item should not be used to fulfil the request.
//
// Because this is only a private cache, 'public' and 'private' in cache-control aren't
// significant. Similarly, smax-age isn't used.
type State int

const (
	// Uncached indicates that the item is not cached.
	Uncached State = iota

	// Stale indicates that the cached item needs validating before it is returned.
	Stale

	// Fresh indicates the cached item can be returned.
	Fresh

	// Transparent indicates the cached item should not be used to fulfil the request.
	Transparent
)

// String returns a string representation of State.
func (s State) String() string {
	switch s {
	case Uncached:
		return "uncached"
	case Stale:
		return "stale"
	case Fresh:
		return "fresh"
	case Transparent:
		return "transparent"
	default:
		return "unknown"
	}
}

// XFromCache is the header added to responses that are returned from the cache.
const XFromCache = "X-From-Stream"

// Downloader encapsulates downloading a file from a URL, using a local
// disk cache if possible. Downloader.Get returns either a filepath to the
// already-downloaded file, or a stream of the download in progress, or an
// error. If a stream is returned, the Downloader cache is updated when the
// stream is fully consumed (this can be observed by the closing of the
// channel returned by [streamcache.Stream.Filled]).
//
// To be extra clear about that last point: the caller must consume the
// stream returned by Downloader.Get, or the cache will not be written.
type Downloader struct {
	// c is the HTTP client used to make requests.
	c *http.Client

	// cache implements the on-disk cache. If nil, caching is disabled.
	// It will be created in dlDir.
	cache *cache

	// name is a user-friendly name, such as a source handle like @data.
	name string

	// url is the URL of the download. It is parsed in downloader.New,
	// thus is guaranteed to be valid.
	url string

	// dlDir is the directory in which the download cache is stored.
	dlDir string

	// mu guards concurrent access to the Downloader.
	mu sync.Mutex

	// continueOnError, if true, indicates that the downloader
	// should server the cached file if a refresh attempt fails.
	continueOnError bool

	// markCachedResponses, if true, indicates that responses returned from the
	// cache will be given an extra header, X-From-cache.
	markCachedResponses bool
}

// New returns a new Downloader for url that caches downloads in dlDir.
// Arg name is a user-friendly label, such as a source handle like @data.
// The name may show up in logs, or progress indicators etc.
func New(name string, c *http.Client, dlURL, dlDir string) (*Downloader, error) {
	_, err := url.ParseRequestURI(dlURL)
	if err != nil {
		return nil, errz.Wrap(err, "invalid download URL")
	}

	if dlDir, err = filepath.Abs(dlDir); err != nil {
		return nil, errz.Err(err)
	}

	dl := &Downloader{
		name:                name,
		c:                   c,
		url:                 dlURL,
		markCachedResponses: true,
		continueOnError:     true,
		dlDir:               dlDir,
	}

	return dl, nil
}

// Get attempts to get the remote file, returning either the filepath of the
// already-cached file in dlFile, or a stream of a newly-started download in
// dlStream, or an error. Exactly one of the return values will be non-nil.
//
//   - If dlFile is non-empty, it is the filepath on disk of the cached download,
//     and dlStream and err are nil. However, depending on OptContinueOnError,
//     dlFile may be the path to a stale download. If the cache is stale and a
//     transport error occurs during refresh, and OptContinueOnError is true,
//     the previous cached download is returned. If OptContinueOnError is false,
//     the transport error is returned, and dlFile is empty. The caller can also
//     check the cache state via [Downloader.State].
//   - If dlStream is non-nil, it is a stream of the download in progress, and
//     dlFile is empty. The cache is updated when the stream has been completely
//     consumed. If the stream is not consumed, the cache is not updated. If an
//     error occurs reading from the stream, the cache is also not updated: this
//     means that the cache may still contain the previous (stale) download.
//   - If err is non-nil, there was an unrecoverable problem (e.g. a transport
//     error, and there's no previous cache) and the download is unavailable.
//
// Get consults the context for options. In particular, it makes use of OptCache
// and OptContinueOnError.
func (dl *Downloader) Get(ctx context.Context) (dlFile string, dlStream *streamcache.Stream, err error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	o := options.FromContext(ctx)
	dl.continueOnError = OptContinueOnError.Get(o)
	if OptCache.Get(o) {
		dl.cache = &cache{dir: dl.dlDir}
	}

	req := dl.mustRequest(ctx)
	lg.FromContext(ctx).Debug("Get download", lga.URL, dl.url)
	return dl.get(req)
}

// get contains the main logic for getting the download.
func (dl *Downloader) get(req *http.Request) (dlFile string, //nolint:gocognit,cyclop
	dlStream *streamcache.Stream, err error,
) {
	ctx := req.Context()
	log := lg.FromContext(ctx)

	var fpBody string
	if dl.cache != nil {
		_, fpBody, _ = dl.cache.paths(req)
	}

	state := dl.state(req)
	if state == Fresh && fpBody != "" {
		// The cached response is fresh, so we can return it.
		return fpBody, nil, nil
	}

	cacheable := dl.isCacheable(req)
	var cachedResp *http.Response
	if cacheable {
		cachedResp, err = dl.cache.get(req.Context(), req) //nolint:bodyclose
	}

	var resp *http.Response
	if cacheable && cachedResp != nil && err == nil { //nolint:nestif
		if dl.markCachedResponses {
			cachedResp.Header.Set(XFromCache, "1")
		}

		if varyMatches(cachedResp, req) {
			// Can only use cached value if the new request doesn't Vary significantly
			freshness := getFreshness(cachedResp.Header, req.Header)
			if freshness == Fresh && fpBody != "" {
				lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, cachedResp.Body)
				return fpBody, nil, nil
			}

			if freshness == Stale {
				var req2 *http.Request
				// Append validators if caller hasn't already done so
				etag := cachedResp.Header.Get("etag")
				if etag != "" && req.Header.Get("etag") == "" {
					req2 = cloneRequest(req)
					req2.Header.Set("if-none-match", etag)
				}
				lastModified := cachedResp.Header.Get("last-modified")
				if lastModified != "" && req.Header.Get("last-modified") == "" {
					if req2 == nil {
						req2 = cloneRequest(req)
					}
					req2.Header.Set("if-modified-since", lastModified)
				}
				if req2 != nil {
					req = req2
				}
			}
		}

		resp, err = dl.do(req) //nolint:bodyclose
		switch {
		case err == nil && req.Method == http.MethodGet && resp.StatusCode == http.StatusNotModified:
			// Replace the 304 response with the one from cache, but update with some new headers
			endToEndHeaders := getEndToEndHeaders(resp.Header)
			for _, header := range endToEndHeaders {
				cachedResp.Header[header] = resp.Header[header]
			}
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
			resp = cachedResp

		case fpBody != "" && (err != nil || resp.StatusCode >= 500) &&
			req.Method == http.MethodGet && canStaleOnError(cachedResp.Header, req.Header):
			// In case of transport failure canStaleOnError returns true,
			// return the stale cached download.
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, cachedResp.Body)
			log.Warn("Returning cached response due to transport failure", lga.Err, err)
			return fpBody, nil, nil

		default:
			if err != nil && resp != nil && resp.StatusCode != http.StatusOK {
				log.Warn("Unexpected HTTP status from server; will serve from cache if possible",
					lga.Err, err, lga.Status, resp.StatusCode)

				if fp := dl.cacheFileOnError(req, err); fp != "" {
					lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
					return fp, nil, nil
				}
			}

			if err != nil {
				lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					return fp, nil, nil
				}

				return "", nil, err
			}

			if resp.StatusCode != http.StatusOK {
				lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
				err = errz.Errorf("download: unexpected HTTP status: %s", httpz.StatusText(resp.StatusCode))
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					return fp, nil, nil
				}

				return "", nil, err
			}
		}
	} else {
		reqCacheControl := parseCacheControl(req.Header)
		if _, ok := reqCacheControl["only-if-cached"]; ok {
			resp = newGatewayTimeoutResponse(req)
		} else {
			resp, err = dl.do(req) //nolint:bodyclose
			if err != nil {
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					return fp, nil, nil
				}

				return "", nil, err
			}
		}
	}

	if cacheable && canStore(parseCacheControl(req.Header), parseCacheControl(resp.Header)) {
		for _, varyKey := range headerAllCommaSepValues(resp.Header, "vary") {
			varyKey = http.CanonicalHeaderKey(varyKey)
			fakeHeader := "X-Varied-" + varyKey
			reqValue := req.Header.Get(varyKey)
			if reqValue != "" {
				resp.Header.Set(fakeHeader, reqValue)
			}
		}

		if resp == cachedResp {
			err = dl.cache.writeHeader(ctx, resp)
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
			if err != nil {
				log.Error("Failed to update cache header", lga.Dir, dl.cache.dir, lga.Err, err)
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					return fp, nil, nil
				}

				return "", nil, err
			}

			if fpBody != "" {
				return fpBody, nil, nil
			}
		} else if cachedResp != nil && cachedResp.Body != nil {
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, cachedResp.Body)
		}

		// OK, this is where the funky stuff happens.
		//
		// First, note that the cache is two-stage: there's a staging cache, and a
		// main cache. The staging cache is used to write the response body to disk,
		// and when the response body is fully consumed, the staging cache is
		// promoted to main, in a sort of atomic-swap-lite. This is done to avoid
		// partially-written cache files in the main cache, and other such nastiness.
		//
		// The responseCacher type is an io.ReadCloser that wraps the response body.
		// As its Read method is called, it writes the body bytes to a staging cache
		// file (and also returns them to the caller). When rCacher encounters io.EOF,
		// it promotes the staging cache to main before returning io.EOF to the
		// caller. If promotion fails, the promotion error (not io.EOF) is returned
		// to the caller. Thus, it is guaranteed that any caller of rCacher's Read
		// method will only receive io.EOF if the cache has been promoted to main.
		//
		// DESIGN NOTE: An earlier version of this code wrapped the resp.Body in a
		// streamcache, created a reader from that streamcache, and filled the cache
		// on a goroutine using that reader, while also returning the streamcache
		// to the Downloader.Get caller (which spawned at least one streamcache
		// reader to perform ingest, and possibly yet more readers to sample the
		// stream). While this approach did work, it had a serious downside: this
		// meant that there would be (at least) two streamcache readers - the cache
		// filler, and the ingester - that would consume the full stream. And thus
		// the streamcache would need to cache the entire download contents in
		// memory, and wouldn't be able to take advantage of the Stream.Seal
		// mechanism for the final reader. It also led to racy situations. Under
		// that mechanism, <-Stream.Filled() could be reached by one of the
		// non-cache-filling readers before the cache was written to disk. And thus,
		// there was required a bunch of ugly code to synchronize with cache fill
		// completion. The current approach guarantees that the cache is filled by
		// the time ANY streamcache readers reaches io.EOF, as observed by
		// <-Stream.Filled().
		var rCacher *responseCacher
		if rCacher, err = dl.cache.newResponseCacher(ctx, resp); err != nil {
			return "", nil, err
		}

		// And now we wrap rCacher in a streamcache: any streamcache readers will
		// only receive io.EOF when/if the staging cache has been promoted to main.
		dlStream = streamcache.New(rCacher)
		return "", dlStream, nil
	}

	// The response is not cacheable, so we can just wrap resp.Body in a
	// streamcache and return it.
	dlStream = streamcache.New(resp.Body)
	resp.Body = nil // Unnecessary, but just to be explicit.
	return "", dlStream, nil
}

// do executes the request.
func (dl *Downloader) do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	log := lg.FromContext(ctx)
	bar := progress.FromContext(ctx).NewWaiter(dl.name + ": start download")
	start := time.Now()
	resp, err := dl.c.Do(req)
	logResp(log, req, resp, time.Since(start), err)
	bar.Stop()
	if err != nil {
		// Download timeout errors are typically wrapped in an url.Error, resulting
		// in a message like:
		//
		//  Get "https://example.com": http response header not received within 1ms timeout
		//
		// We want to trim off that `GET "URL"` prefix, but we only do that if
		// there's a wrapped error beneath (which should be the case).
		if errz.Has[*url.Error](err) && errors.Is(err, context.DeadlineExceeded) {
			if e := errors.Unwrap(err); e != nil {
				err = e
			}
		}
		return nil, err
	}

	if resp.Body != nil && resp.Body != http.NoBody {
		r := progress.NewReader(req.Context(), dl.name+": download", resp.ContentLength, resp.Body)
		if rc, ok := r.(io.ReadCloser); ok {
			resp.Body = rc
		}
	}
	return resp, nil
}

// mustRequest creates a new request from dl.url. The url has already been
// parsed in [downloader.New], so it's safe to use.
func (dl *Downloader) mustRequest(ctx context.Context) *http.Request {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.url, nil)
	if err != nil {
		lg.FromContext(ctx).Error("Failed to create request", lga.URL, dl.url, lga.Err, err)
		panic(err)
	}
	return req
}

// Clear deletes the cache.
func (dl *Downloader) Clear(ctx context.Context) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	if dl.cache == nil {
		return nil
	}

	return dl.cache.clear(ctx)
}

// State returns the Downloader's cache state.
func (dl *Downloader) State(ctx context.Context) State {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return dl.state(dl.mustRequest(ctx))
}

// state returns the cache state for the given request. It reads the cached
// response headers (if present) and evaluates freshness according to HTTP
// cache-control semantics. Returns [Uncached] if there is no cache or if
// the cache cannot be read.
func (dl *Downloader) state(req *http.Request) State {
	if !dl.isCacheable(req) {
		return Uncached
	}

	ctx := req.Context()
	log := lg.FromContext(ctx)

	if !dl.cache.exists(req) {
		return Uncached
	}

	fpHeader, _, _ := dl.cache.paths(req)
	f, err := os.Open(fpHeader)
	if err != nil {
		log.Error(msgCloseCacheHeaderFile, lga.File, fpHeader, lga.Err, err)
		return Uncached
	}

	defer lg.WarnIfCloseError(log, msgCloseCacheHeaderFile, f)

	cachedResp, err := httpz.ReadResponseHeader(bufio.NewReader(f), nil) //nolint:bodyclose
	if err != nil {
		log.Error("Failed to read cached response header", lga.Err, err)
		return Uncached
	}

	return getFreshness(cachedResp.Header, req.Header)
}

// CacheFile returns the path to the cached file, if it exists. If there's
// a download in progress ([Downloader.Get] returned a stream), then CacheFile
// may return the filepath to the previously cached file. The caller should
// wait on any previously returned download stream to complete to ensure
// that the returned filepath is that of the current download. The caller
// can also check the cache state via [Downloader.State].
func (dl *Downloader) CacheFile(ctx context.Context) (fp string, err error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.cache == nil {
		return "", errz.Errorf("cache doesn't exist for: %s", dl.url)
	}

	req := dl.mustRequest(ctx)
	if !dl.cache.exists(req) {
		return "", errz.Errorf("no cache for: %s", dl.url)
	}
	_, fp, _ = dl.cache.paths(req)
	return fp, nil
}

// cacheFileOnError returns the path to the cached file, if it exists,
// and is allowed to be returned on a refresh error. If not, empty
// string is returned.
func (dl *Downloader) cacheFileOnError(req *http.Request, err error) (fp string) {
	if req == nil {
		return ""
	}

	if dl.cache == nil {
		return ""
	}

	if !dl.continueOnError {
		return ""
	}

	if !dl.cache.exists(req) {
		return ""
	}

	_, fp, _ = dl.cache.paths(req)
	lg.FromContext(req.Context()).Warn("Returning possibly stale cached response due to download refresh error",
		lga.Err, err, lga.Path, fp)
	return fp
}

// Checksum returns the checksum of the cached download, if available.
func (dl *Downloader) Checksum(ctx context.Context) (sum checksum.Checksum, ok bool) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.cache == nil {
		return "", false
	}

	req := dl.mustRequest(ctx)
	return dl.cache.cachedChecksum(req)
}

// isCacheable returns true if the request can be served from or stored to
// the cache. A request is cacheable if caching is enabled, the HTTP method
// is GET or HEAD, and the request does not include a Range header (partial
// content requests are not cached).
func (dl *Downloader) isCacheable(req *http.Request) bool {
	if dl.cache == nil {
		return false
	}
	return (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Header.Get("range") == ""
}
