// Package download provides a mechanism for getting files from
// HTTP URLs, making use of a mostly RFC-compliant cache.
//
// FIXME: move download to internal/download, because its use
// is so specialized?
//
// Acknowledgement: This package is a heavily customized fork
// of https://github.com/gregjones/httpcache, via bitcomplete/download.
package download

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
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

// XFromCache is the header added to responses that are returned from the cache.
const XFromCache = "X-From-Cache"

const msgNilDestWriter = "nil dest writer from download handler; returning"

// Opt is a configuration option for creating a new Download.
type Opt func(t *Download)

// OptMarkCacheResponses configures a Download by setting
// Download.markCachedResponses to true.
func OptMarkCacheResponses(markCachedResponses bool) Opt {
	return func(t *Download) {
		t.markCachedResponses = markCachedResponses
	}
}

// OptDisableCaching disables the cache.
func OptDisableCaching(disable bool) Opt {
	return func(t *Download) {
		t.disableCaching = disable
	}
}

// Download encapsulates downloading a file from a URL, using a local
// disk cache if possible.
type Download struct {
	// FIXME: Does Download need a sync.Mutex?

	// name is a user-friendly name, such as a source handle like @data.
	name string

	// url is the URL of the download. It is parsed in download.New,
	// thus is guaranteed to be valid.
	url string

	c *http.Client

	cache *cache

	// markCachedResponses, if true, indicates that responses returned from the
	// cache will be given an extra header, X-From-cache.
	markCachedResponses bool

	disableCaching bool
}

// New returns a new Download for url that writes to cacheDir.
// Name is a user-friendly name, such as a source handle like @data.
// The name may show up in logs, or progress indicators etc.
// If c is nil, httpz.NewDefaultClient is used.
func New(name string, c *http.Client, dlURL, cacheDir string, opts ...Opt) (*Download, error) {
	_, err := url.ParseRequestURI(dlURL)
	if err != nil {
		return nil, errz.Wrap(err, "invalid download URL")
	}
	if c == nil {
		c = httpz.NewDefaultClient()
	}

	if cacheDir, err = filepath.Abs(cacheDir); err != nil {
		return nil, errz.Err(err)
	}

	t := &Download{
		name:                name,
		c:                   c,
		url:                 dlURL,
		markCachedResponses: true,
		disableCaching:      false,
	}
	for _, opt := range opts {
		opt(t)
	}

	if !t.disableCaching {
		t.cache = &cache{dir: cacheDir}
	}

	return t, nil
}

// Get gets the download, invoking Handler as appropriate.
func (dl *Download) Get(ctx context.Context, h Handler) {
	req := dl.mustRequest(ctx)
	dl.get(req, h)
}

// get contains the main logic for getting the download. It invokes Handler
// as appropriate.
func (dl *Download) get(req *http.Request, h Handler) { //nolint:funlen,gocognit
	ctx := req.Context()
	log := lg.FromContext(ctx)
	log.Debug("Get download", lga.URL, dl.url)
	_, fpBody, _ := dl.cache.paths(req)

	state := dl.state(req)
	if state == Fresh {
		// The cached response is fresh, so we can return it.
		h.Cached(fpBody)
		return
	}

	cacheable := dl.isCacheable(req)
	var err error
	var cachedResp *http.Response
	if cacheable {
		cachedResp, err = dl.cache.get(req.Context(), req) //nolint:bodyclose
	} else {
		// Need to invalidate an existing value
		if err = dl.cache.clear(req.Context()); err != nil {
			h.Error(err)
			return
		}
	}

	var resp *http.Response
	if cacheable && cachedResp != nil && err == nil { //nolint:nestif
		if dl.markCachedResponses {
			cachedResp.Header.Set(XFromCache, "1")
		}

		if varyMatches(cachedResp, req) {
			// Can only use cached value if the new request doesn't Vary significantly
			freshness := getFreshness(cachedResp.Header, req.Header)
			if freshness == Fresh {
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
		if err == nil && req.Method == http.MethodGet && resp.StatusCode == http.StatusNotModified {
			// Replace the 304 response with the one from cache, but update with some new headers
			endToEndHeaders := getEndToEndHeaders(resp.Header)
			for _, header := range endToEndHeaders {
				cachedResp.Header[header] = resp.Header[header]
			}
			lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
			resp = cachedResp
		} else if (err != nil ||
			(cachedResp != nil &&
				resp.StatusCode >= 500)) &&
			req.Method == http.MethodGet &&
			canStaleOnError(cachedResp.Header, req.Header) {
			// In case of transport failure and stale-if-error activated, returns cached content
			// when available
			log.Warn("Returning cached response due to transport failure", lga.Err, err)
			h.Cached(fpBody)
			return
		} else {
			if err != nil || resp.StatusCode != http.StatusOK {
				lg.WarnIfError(log, msgDeleteCache, dl.cache.clear(req.Context()))
			}
			if err != nil {
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
			if err = dl.cache.write(ctx, resp, true, nil); err != nil {
				log.Error("Failed to update cache header", lga.Dir, dl.cache.dir, lga.Err, err)
				h.Error(err)
				return
			}
			h.Cached(fpBody)
			return
		}

		// I'm not sure if this logic is even reachable?
		destWrtr := h.Uncached()
		if destWrtr == nil {
			log.Warn(msgNilDestWriter)
			return
		}

		defer lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, resp.Body)
		if err = dl.cache.write(req.Context(), resp, false, destWrtr); err != nil {
			log.Error("Failed to write download cache", lga.Dir, dl.cache.dir, lga.Err, err)
			// We don't need to explicitly call Handler.Error here, because the caller is
			// informed via destWrtr.Error, which has already been invoked by cache.write.
		}
		return
	}

	lg.WarnIfError(log, "Delete resp cache", dl.cache.clear(req.Context()))

	// It's not cacheable, so we need to write it to the destWrtr,
	// and skip the cache.
	destWrtr := h.Uncached()
	if destWrtr == nil {
		// Shouldn't happen.
		log.Warn(msgNilDestWriter)
		return
	}

	cr := contextio.NewReader(ctx, resp.Body)
	defer lg.WarnIfCloseError(log, lgm.CloseHTTPResponseBody, cr.(io.ReadCloser))
	_, err = io.Copy(destWrtr, cr)
	if err != nil {
		log.Error("Failed to copy download to dest writer", lga.Err, err)
		destWrtr.Error(err)
		return
	}
	if err = destWrtr.Close(); err != nil {
		log.Error("Failed to close dest writer", lga.Err, err)
	}
}

// do executes the request.
func (dl *Download) do(req *http.Request) (*http.Response, error) {
	bar := progress.FromContext(req.Context()).NewWaiter(dl.name+": start download", true)
	resp, err := dl.c.Do(req)
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
func (dl *Download) mustRequest(ctx context.Context) *http.Request {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.url, nil)
	if err != nil {
		lg.FromContext(ctx).Error("Failed to create request", lga.URL, dl.url, lga.Err, err)
		panic(err)
	}
	return req
}

// Clear deletes the cache.
func (dl *Download) Clear(ctx context.Context) error {
	if dl.cache != nil {
		return dl.cache.clear(ctx)
	}
	return nil
}

// State returns the Download's cache state.
func (dl *Download) State(ctx context.Context) State {
	return dl.state(dl.mustRequest(ctx))
}

func (dl *Download) state(req *http.Request) State {
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

// Checksum returns the checksum of the cached download, if available.
func (dl *Download) Checksum(ctx context.Context) (sum checksum.Checksum, ok bool) {
	if dl.cache == nil {
		return "", false
	}

	req := dl.mustRequest(ctx)
	return dl.cache.cachedChecksum(req)
}

func (dl *Download) isCacheable(req *http.Request) bool {
	if dl.disableCaching {
		return false
	}
	return (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Header.Get("range") == ""
}
