// Package download provides a http.RoundTripper implementation that
// works as a mostly RFC-compliant cache for http responses.
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
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"io"
	"net/http"
	"os"
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

// XFromCache is the header added to responses that are returned from the cache
const XFromCache = "X-From-Cache"

// Opt is a configuration option for creating a new Download.
type Opt func(t *Download)

// OptMarkCacheResponses configures a Download by setting
// Download.markCachedResponses to true.
func OptMarkCacheResponses(markCachedResponses bool) Opt {
	return func(t *Download) {
		t.markCachedResponses = markCachedResponses
	}
}

// OptInsecureSkipVerify configures a Download to skip TLS verification.
func OptInsecureSkipVerify(insecureSkipVerify bool) Opt {
	return func(t *Download) {
		t.InsecureSkipVerify = insecureSkipVerify
	}
}

// OptDisableCaching disables the cache.
func OptDisableCaching(disable bool) Opt {
	return func(t *Download) {
		t.disableCaching = disable
	}
}

// OptUserAgent sets the User-Agent header on requests.
func OptUserAgent(userAgent string) Opt {
	return func(t *Download) {
		t.userAgent = userAgent
	}
}

// Download is aan implementation of http.RoundTripper that will return values from a cache
// where possible (avoiding a network request) and will additionally add validators (etag/if-modified-since)
// to repeated requests allowing servers to return 304 / Not Modified
type Download struct {
	// FIXME: Does Download need a sync.Mutex?

	// FIXME: implement url mechanism
	// url is the URL of the download.
	url string

	// The RoundTripper interface actually used to make requests
	// If nil, http.DefaultTransport is used.
	transport http.RoundTripper

	// respCache is the cache used to store responses.
	respCache *RespCache

	// markCachedResponses, if true, indicates that responses returned from the
	// cache will be given an extra header, X-From-Cache
	markCachedResponses bool

	InsecureSkipVerify bool

	userAgent string

	disableCaching bool
}

// New returns a new Download that uses cacheDir as the cache directory.
func New(url, cacheDir string, opts ...Opt) *Download {
	t := &Download{
		url:                 url,
		markCachedResponses: true,
		disableCaching:      false,
		InsecureSkipVerify:  false,
	}
	for _, opt := range opts {
		opt(t)
	}

	if !t.disableCaching {
		t.respCache = NewRespCache(cacheDir)
	}
	return t
}

// Handler is a callback invoked by Download.Get. Exactly one of the
// handler functions will be invoked, one time.
type Handler struct {
	// Cached is invoked when the download is already cached on disk. The
	// fp arg is the path to the downloaded file.
	Cached func(fp string)

	// Uncached is invoked when the download is not cached. The handler must
	// return an io.WriterCloser, which the download contents will be written
	// to (as well as being written to the disk cache). On success, the dest
	// io.WriteCloser is closed. If an error occurs during download or
	// writing, errFn is invoked, and dest is not closed.
	Uncached func() (dest io.WriteCloser, errFn func(error))

	// Error is invoked if an
	Error func(err error)
}

// Get gets the download, invoking Handler as appropriate.
func (dl *Download) Get(ctx context.Context, h Handler) {
	req, err := dl.newRequest(ctx, dl.url)
	if err != nil {
		h.Error(err)
		return
	}

	dl.get(req, h)
}

func (dl *Download) get(req *http.Request, cb Handler) {
	ctx := req.Context()
	log := lg.FromContext(ctx)
	log.Info("Fetching download", lga.URL, req.URL.String())
	_, fpBody := dl.respCache.Paths(req)

	if dl.state(req) == Fresh {
		cb.Cached(fpBody)
		return
	}

	var err error
	cacheable := dl.isCacheable(req)
	var cachedResp *http.Response
	if cacheable {
		cachedResp, err = dl.respCache.Get(req.Context(), req)
	} else {
		// Need to invalidate an existing value
		if err = dl.respCache.Clear(req.Context()); err != nil {
			cb.Error(err)
			return
		}
	}

	var resp *http.Response
	if cacheable && cachedResp != nil && err == nil {
		if dl.markCachedResponses {
			cachedResp.Header.Set(XFromCache, "1")
		}

		if varyMatches(cachedResp, req) {
			// Can only use cached value if the new request doesn't Vary significantly
			freshness := getFreshness(cachedResp.Header, req.Header)
			if freshness == Fresh {
				cb.Cached(fpBody)
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

		resp, err = dl.execRequest(req)
		if err == nil && req.Method == http.MethodGet && resp.StatusCode == http.StatusNotModified {
			// Replace the 304 response with the one from cache, but update with some new headers
			endToEndHeaders := getEndToEndHeaders(resp.Header)
			for _, header := range endToEndHeaders {
				cachedResp.Header[header] = resp.Header[header]
			}
			resp = cachedResp
		} else if (err != nil || (cachedResp != nil && resp.StatusCode >= 500)) &&
			req.Method == http.MethodGet && canStaleOnError(cachedResp.Header, req.Header) {
			// In case of transport failure and stale-if-error activated, returns cached content
			// when available
			log.Warn("Returning cached response due to transport failure", lga.Err, err)
			cb.Cached(fpBody)
			return
		} else {
			if err != nil || resp.StatusCode != http.StatusOK {
				lg.WarnIfError(log, msgDeleteCache, dl.respCache.Clear(req.Context()))
			}
			if err != nil {
				cb.Error(err)
				return
			}
		}
	} else {
		reqCacheControl := parseCacheControl(req.Header)
		if _, ok := reqCacheControl["only-if-cached"]; ok {
			resp = newGatewayTimeoutResponse(req)
		} else {
			resp, err = dl.execRequest(req)
			if err != nil {
				cb.Error(err)
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

		copyWrtr, errFn := cb.Uncached()
		if copyWrtr == nil {
			log.Warn("nil copy writer from download handler; returning")
			return
		}

		if err = dl.respCache.Write(req.Context(), resp, copyWrtr); err != nil {
			log.Error("failed to write download cache", lga.Err, err)
			errFn(err)
			cb.Error(err)
		}
		return
	} else {
		lg.WarnIfError(log, "Delete resp cache", dl.respCache.Clear(req.Context()))
	}

	// It's not cacheable, so we need to write it to the copyWrtr.
	copyWrtr, errFn := cb.Uncached()
	if copyWrtr == nil {
		log.Warn("nil copy writer from download handler; returning")
		return
	}

	cr := contextio.NewReader(ctx, resp.Body)
	_, err = io.Copy(copyWrtr, cr)
	if err != nil {
		errFn(err)
		cb.Error(err)
		return
	}
	if err = copyWrtr.Close(); err != nil {
		cb.Error(err)
		return
	}

	return
}

// Close frees any resources held by the Download. It does not delete
// the cache from disk. For that, see Download.Clear.
func (dl *Download) Close() error {
	if dl.respCache != nil {
		return dl.respCache.Close()
	}
	return nil
}

// execRequest executes the request.
func (dl *Download) execRequest(req *http.Request) (*http.Response, error) {
	if dl.transport == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return dl.transport.RoundTrip(req)
}

func (dl *Download) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		lg.FromContext(ctx).Error("Failed to create request", lga.URL, url, lga.Err, err)
		return nil, err
	}
	if dl.userAgent != "" {
		req.Header.Set("User-Agent", dl.userAgent)
	}
	return req, nil
}

func (dl *Download) getClient() *http.Client {
	return ioz.NewHTTPClient(dl.InsecureSkipVerify)
}

// Clear deletes the cache.
func (dl *Download) Clear(ctx context.Context) error {
	if dl.respCache != nil {
		return dl.respCache.Clear(ctx)
	}
	return nil
}

// State returns the Download's cache state.
func (dl *Download) State(ctx context.Context) State {
	req, err := dl.newRequest(ctx, dl.url)
	if err != nil {
		return Uncached
	}
	return dl.state(req)
}

func (dl *Download) state(req *http.Request) State {
	if !dl.isCacheable(req) {
		return Uncached
	}

	ctx := req.Context()
	log := lg.FromContext(ctx)

	if !dl.respCache.Exists(req) {
		return Uncached
	}

	fpHeader, _ := dl.respCache.Paths(req)
	f, err := os.Open(fpHeader)
	if err != nil {
		log.Error("Failed to open cached response header file", lga.File, fpHeader, lga.Err, err)
		return Uncached
	}

	defer lg.WarnIfCloseError(log, "Close cached response header file", f)

	cachedResp, err := readResponseHeader(bufio.NewReader(f), nil)
	if err != nil {
		log.Error("Failed to read cached response header", lga.Err, err)
		return Uncached
	}

	return getFreshness(cachedResp.Header, req.Header)
}

func (dl *Download) isCacheable(req *http.Request) bool {
	if dl.disableCaching {
		return false
	}
	return (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Header.Get("range") == ""
}