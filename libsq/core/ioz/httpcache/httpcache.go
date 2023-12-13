// Package httpcache provides a http.RoundTripper implementation that
// works as a mostly RFC-compliant cache for http responses.
//
// FIXME: move httpcache to internal/httpcache, because its use
// is so specialized?
//
// Acknowledgement: This package is a heavily customized fork
// of https://github.com/gregjones/httpcache, via bitcomplete/httpcache.
package httpcache

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

const (
	stale = iota
	fresh
	transparent
	// XFromCache is the header added to responses that are returned from the cache
	XFromCache = "X-From-Cache"
)

// Opt is a configuration option for creating a new Transport.
type Opt func(t *Transport)

// OptMarkCacheResponses configures a Transport by setting
// Transport.markCachedResponses to true.
func OptMarkCacheResponses(markCachedResponses bool) Opt {
	return func(t *Transport) {
		t.markCachedResponses = markCachedResponses
	}
}

// OptInsecureSkipVerify configures a Transport to skip TLS verification.
func OptInsecureSkipVerify(insecureSkipVerify bool) Opt {
	return func(t *Transport) {
		t.InsecureSkipVerify = insecureSkipVerify
	}
}

// OptDisableCaching disables the cache.
func OptDisableCaching(disable bool) Opt {
	return func(t *Transport) {
		t.disableCaching = disable
	}
}

// OptUserAgent sets the User-Agent header on requests.
func OptUserAgent(userAgent string) Opt {
	return func(t *Transport) {
		t.userAgent = userAgent
	}
}

// Transport is an implementation of http.RoundTripper that will return values from a cache
// where possible (avoiding a network request) and will additionally add validators (etag/if-modified-since)
// to repeated requests allowing servers to return 304 / Not Modified
type Transport struct {
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

// NewTransport returns a new Transport with the provided Cache and options. If
// KeyFunc is not specified in opts then DefaultKeyFunc is used.
func NewTransport(cacheDir string, opts ...Opt) *Transport {
	t := &Transport{
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

type Handler struct {
	Cached   func(cachedFilepath string) error
	Uncached func() (wc io.WriteCloser, errFn func(error), err error)
	Error    func(err error)
}

func (t *Transport) Fetch(ctx context.Context, dlURL string, h Handler) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		h.Error(err)
		return
	}
	if t.userAgent != "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	t.FetchWith(req, h)
}

func (t *Transport) FetchWith(req *http.Request, cb Handler) {
	ctx := req.Context()
	log := lg.FromContext(ctx)
	log.Info("Fetching download", lga.URL, req.URL.String())
	_ = log
	_, fpBody := t.respCache.Paths(req)

	if t.IsFresh(req) {
		_ = cb.Cached(fpBody)
		return
	}

	var err error
	cacheable := t.isCacheable(req)
	var cachedResp *http.Response
	if cacheable {
		cachedResp, err = t.respCache.Get(req.Context(), req)
	} else {
		// Need to invalidate an existing value
		if err = t.respCache.Delete(req.Context()); err != nil {
			cb.Error(err)
			return
		}
	}

	transport := t.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	var resp *http.Response
	if cacheable && cachedResp != nil && err == nil {
		if t.markCachedResponses {
			cachedResp.Header.Set(XFromCache, "1")
		}

		if varyMatches(cachedResp, req) {
			// Can only use cached value if the new request doesn't Vary significantly
			freshness := getFreshness(cachedResp.Header, req.Header)
			if freshness == fresh {
				_ = cb.Cached(fpBody)
				return
			}

			if freshness == stale {
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

		// FIXME: Use an http client here
		resp, err = transport.RoundTrip(req)
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
				lg.WarnIfError(log, msgDeleteCache, t.respCache.Delete(req.Context()))
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
			resp, err = transport.RoundTrip(req)
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

		copyWrtr, errFn, err := cb.Uncached()
		if err != nil {
			cb.Error(err)
			return
		}

		if err = t.respCache.Write(req.Context(), resp, copyWrtr); err != nil {
			log.Error("failed to write download cache", lga.Err, err)
			errFn(err)
			cb.Error(err)
		}
		return
	} else {
		lg.WarnIfError(log, "Delete resp cache", t.respCache.Delete(req.Context()))
	}

	// It's not cacheable, so we need to write it to the copyWrtr.
	copyWrtr, errFn, err := cb.Uncached()
	if err != nil {
		cb.Error(err)
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

func (t *Transport) getClient() *http.Client {
	return ioz.NewHTTPClient(t.InsecureSkipVerify)
}

// Delete deletes the cache.
func (t *Transport) Delete(ctx context.Context) error {
	if t.respCache != nil {
		return t.respCache.Delete(ctx)
	}
	return nil
}

// IsCached returns true if there is a cache entry for req. This does not
// guarantee that the cache entry is fresh. See also: [Transport.IsFresh].
func (t *Transport) IsCached(req *http.Request) bool {
	if t.disableCaching {
		return false
	}
	return t.respCache.Exists(req)
}

// IsFresh returns true if there is a fresh cache entry for req.
func (t *Transport) IsFresh(req *http.Request) bool {
	ctx := req.Context()
	log := lg.FromContext(ctx)

	if !t.isCacheable(req) {
		return false
	}

	if !t.respCache.Exists(req) {
		return false
	}

	fpHeader, _ := t.respCache.Paths(req)
	f, err := os.Open(fpHeader)
	if err != nil {
		log.Error("Failed to open cached response header file", lga.File, fpHeader, lga.Err, err)
		return false
	}

	defer lg.WarnIfCloseError(log, "Close cached response header", f)

	cachedResp, err := readResponseHeader(bufio.NewReader(f), nil)
	if err != nil {
		log.Error("Failed to read cached response", lga.Err, err)
		return false
	}

	freshness := getFreshness(cachedResp.Header, req.Header)
	return freshness == fresh
}

func (t *Transport) isCacheable(req *http.Request) bool {
	if t.disableCaching {
		return false
	}
	return (req.Method == http.MethodGet || req.Method == http.MethodHead) && req.Header.Get("range") == ""
}
