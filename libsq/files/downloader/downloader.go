// Package downloader provides a mechanism for getting files from
// HTTP/S URLs, making use of a mostly RFC-compliant cache.
//
// The entrypoint is downloader.New.
//
// Acknowledgement: This package is a heavily customized fork
// of https://github.com/gregjones/httpcache, via bitcomplete/download.
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

var OptContinueOnError = options.NewBool(
	"download.refresh.continue-on-error",
	"",
	false,
	0,
	true,
	"Continue with stale download if refresh fails",
	`Continue with stale download if refresh fails. This option applies if
a download is in the cache, but is considered stale, and a refresh attempt fails.
If set to true, the refresh error is logged, and the stale download is returned.
This is a sort of "Airplane Mode" for downloads: when true, sq continues with
the cached download when the network is unavailable. If false, an error is
returned instead.`,
	options.TagSource,
)

var OptCache = options.NewBool(
	"download.cache",
	"",
	false,
	0,
	true,
	"Cache downloads",
	`Cache downloaded remote files. When false, the cache is not used and
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
// disk cache if possible.
type Downloader struct {
	mu sync.Mutex

	// name is a user-friendly name, such as a source handle like @data.
	name string

	// url is the URL of the download. It is parsed in download.New,
	// thus is guaranteed to be valid.
	url string

	// c is the HTTP client used to make requests.
	c *http.Client

	// continueOnError, if true, indicates that the downloader
	// should server the cached file if a refresh attempt fails.
	continueOnError bool

	// dlDir is the directory in which the download cache is stored.
	dlDir string

	// cache implements the on-disk cache. If nil, caching is disabled.
	// It will be created in dlDir.
	cache *cache

	// markCachedResponses, if true, indicates that responses returned from the
	// cache will be given an extra header, X-From-cache.
	markCachedResponses bool

	// dlStream is the streamcache.Stream that is returned Handler for an
	// active download. This field is used by Downloader.Filesize. It is
	// reset to nil on each call to Downloader.Get.
	dlStream *streamcache.Stream
}

// New returns a new Downloader for url that caches downloads in dlDir..
// Name is a user-friendly name, such as a source handle like @data.
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

// Get gets the download, invoking Handler as appropriate. Exactly
// one of the Handler methods will be invoked, one time. If Handler.Uncached
// is invoked, Get blocks until the download is completed. The download
// resp.Body is written to cache; if an error occurs during cache write,
// the cache write error is logged, but no error is returned.
//
// Get consults the context for options. In particular, it makes
// use of OptCache and OptContinueOnError.
func (dl *Downloader) Get(ctx context.Context, h Handler) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	o := options.FromContext(ctx)
	dl.continueOnError = OptContinueOnError.Get(o)
	if OptCache.Get(o) {
		dl.cache = &cache{dir: dl.dlDir}
	}

	req := dl.mustRequest(ctx)
	lg.FromContext(ctx).Debug("Get download", lga.URL, dl.url)
	dl.get(req, h)
}

// get contains the main logic for getting the download. It invokes Handler
// as appropriate.
func (dl *Downloader) get(req *http.Request, h Handler) { //nolint:gocognit,funlen
	ctx := req.Context()
	log := lg.FromContext(ctx)

	dl.dlStream = nil

	var fpBody string
	if dl.cache != nil {
		_, fpBody, _ = dl.cache.paths(req)
	}

	state := dl.state(req)
	if state == Fresh && fpBody != "" {
		// The cached response is fresh, so we can return it.
		h.Cached(fpBody)
		return
	}

	cacheable := dl.isCacheable(req)
	var err error
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
				h.Cached(fpBody)
				return
			}

			if freshness == Stale {
				var req2 *http.Request
				// Add validators if caller hasn't already done so
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

		case fpBody != "" && (err != nil || (resp.StatusCode >= 500)) &&
			req.Method == http.MethodGet && canStaleOnError(cachedResp.Header, req.Header):
			// In case of transport failure and stale-if-error activated, returns cached content
			// when available
			log.Warn("Returning cached response due to transport failure", lga.Err, err)
			h.Cached(fpBody)
			return

		default:
			if err != nil && resp != nil && resp.StatusCode != http.StatusOK {
				log.Warn("Unexpected HTTP status from server; will serve from cache if possible",
					lga.Err, err, lga.Status, resp.StatusCode)

				if fp := dl.cacheFileOnError(req, err); fp != "" {
					h.Cached(fp)
					return
				}
			}

			if err != nil {
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					h.Cached(fp)
					return
				}
				h.Error(err)
				return
			}

			if resp.StatusCode != http.StatusOK {
				err = errz.Errorf("download: unexpected HTTP status: %s", httpz.StatusText(resp.StatusCode))
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					h.Cached(fp)
					return
				}

				h.Error(err)
				return
			}
		}
	} else {
		reqCacheControl := parseCacheControl(req.Header)
		if _, ok := reqCacheControl["only-if-cached"]; ok {
			resp = newGatewayTimeoutResponse(req) //nolint:bodyclose
		} else {
			resp, err = dl.do(req) //nolint:bodyclose
			if err != nil {
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					h.Cached(fp)
					return
				}
				h.Error(err)
				return
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
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
			if err = dl.cache.write(ctx, resp, true); err != nil {
				log.Error("Failed to update cache header", lga.Dir, dl.cache.dir, lga.Err, err)
				if fp := dl.cacheFileOnError(req, err); fp != "" {
					h.Cached(fp)
					return
				}
				h.Error(err)
				return
			}
			if fpBody != "" {
				h.Cached(fpBody)
				return
			}
		}

		dl.dlStream = streamcache.New(resp.Body)
		resp.Body = dl.dlStream.NewReader(ctx)
		h.Uncached(dl.dlStream)

		if err = dl.cache.write(req.Context(), resp, false); err != nil {
			// We don't explicitly call Handler.Error: it would be "illegal" to do so
			// anyway, because the Handler docs state that at most one Handler callback
			// func is ever invoked. The cache write could fail for two reasons:
			// - The download didn't complete successfully: that is, there was an error
			//   reading from resp.Body. In this case, that same error will be propagated
			//   to the Handler via the streamcache.Stream that was provided to Handler.Uncached.
			// - The download completed, but there was a problem writing out the cache
			//   files (header, body, checksum). This is likely a very rare occurrence.
			//   In that case, any previous cache files are left untouched by cache.write,
			//   and all we do is log the error. If the cache is inconsistent, it will
			//   repair itself on next invocation, so it's not a big deal.
			log.Warn("Failed to write download cache", lga.Dir, dl.cache.dir, lga.Err, err)
		}
		return
	}

	// It's not cacheable, so we can just wrap resp.Body in a streamcache
	// and return it.
	dl.dlStream = streamcache.New(resp.Body)
	resp.Body = nil // Unnecessary, but just to be explicit.
	h.Uncached(dl.dlStream)
}

// do executes the request.
func (dl *Downloader) do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	log := lg.FromContext(ctx)
	bar := progress.FromContext(ctx).NewWaiter(dl.name+": start download", true)
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
		resp.Body, _ = r.(io.ReadCloser)
	}
	return resp, nil
}

// mustRequest creates a new request from dl.url. The url has already been
// parsed in download.New, so it's safe to ignore the error.
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

// Filesize returns the size of the downloaded file. This should
// only be invoked after the download has completed or is cached,
// as it may block until the download completes.
func (dl *Downloader) Filesize(ctx context.Context) (int64, error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.dlStream != nil {
		// There's an active download, so we can get the filesize
		// when the download completes.
		size, err := dl.dlStream.Total(ctx)
		return int64(size), err
	}

	if dl.cache == nil {
		return 0, errz.New("download file size not available")
	}

	req := dl.mustRequest(ctx)
	if !dl.cache.exists(req) {
		// It's not in the cache.
		return 0, errz.New("download file size not available")
	}

	// It's in the cache.
	_, fp, _ := dl.cache.paths(req)
	fi, err := os.Stat(fp)
	if err != nil {
		return 0, errz.Wrapf(err, "unable to stat cached download file: %s", fp)
	}

	return fi.Size(), nil
}

// CacheFile returns the path to the cached file, if it exists and has
// been fully downloaded.
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

func (dl *Downloader) isCacheable(req *http.Request) bool {
	if dl.cache == nil {
		return false
	}
	return (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Header.Get("range") == ""
}
