// This file contains HTTP caching helper functions implementing RFC 7234
// (HTTP/1.1 Caching) and RFC 5861 (stale-if-error extension) semantics.
//
// These functions are used by [Downloader] to determine cache freshness,
// validate cached responses, and handle cache-control directives. The
// implementation focuses on private caching (single-user), so directives
// like "public", "private", and "s-maxage" are not significant.

package downloader

import (
	"bufio"
	"bytes"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// errNoDateHeader indicates that the HTTP headers contained no Date header.
// This error is returned by [getDate] when the response lacks a Date header,
// which is required for computing cache age per RFC 7234.
var errNoDateHeader = errors.New("no Date header")

// getDate parses and returns the value of the HTTP Date header from the
// provided response headers.
//
// The Date header (RFC 7231 Section 7.1.1.2) represents the date and time
// at which the message was originated. It is essential for calculating
// the age of a cached response.
//
// Returns [errNoDateHeader] if the Date header is missing, or a parse
// error if the header value cannot be parsed as RFC 1123 format.
func getDate(respHeaders http.Header) (date time.Time, err error) {
	dateHeader := respHeaders.Get("date")
	if dateHeader == "" {
		err = errNoDateHeader
		return date, err
	}

	return time.Parse(time.RFC1123, dateHeader)
}

// realClock is the production implementation of [timer] that uses actual
// system time via [time.Since].
type realClock struct{}

// since returns the elapsed time since d using [time.Since].
func (c *realClock) since(d time.Time) time.Duration {
	return time.Since(d)
}

// timer is an interface for time calculations, allowing tests to inject
// a mock clock for deterministic testing of time-dependent cache logic.
type timer interface {
	// since returns the duration elapsed since the given time.
	since(d time.Time) time.Duration
}

// clock is the package-level timer used for cache age calculations.
// In production this is [realClock]; tests can replace it with a mock.
var clock timer = &realClock{}

// getFreshness determines the freshness state of a cached response based on
// HTTP cache-control directives from both the cached response and new request.
//
// Returns one of three [State] values:
//
//   - [Fresh]: The cached response is valid and can be returned directly
//     without revalidation.
//
//   - [Stale]: The cached response has expired and must be revalidated with
//     the origin server (via conditional request) before use.
//
//   - [Transparent]: The cached response must not be used; a fresh response
//     must be fetched from the origin server.
//
// # Cache-Control Directives Handled
//
// Request directives:
//   - no-cache: Forces [Transparent] (bypass cache entirely)
//   - only-if-cached: Forces [Fresh] (use cache even if stale)
//   - max-age: Limits acceptable response age
//   - min-fresh: Requires response to remain fresh for specified duration
//   - max-stale: Allows accepting stale responses up to specified age
//
// Response directives:
//   - no-cache: Forces [Stale] (always revalidate)
//   - max-age: Specifies freshness lifetime (overrides Expires header)
//
// The Expires header is used as a fallback if max-age is not present.
//
// Since this is a private cache (single-user), the "public", "private", and
// "s-maxage" directives are not significant and are ignored.
func getFreshness(respHeaders, reqHeaders http.Header) (freshness State) {
	respCacheControl := parseCacheControl(respHeaders)
	reqCacheControl := parseCacheControl(reqHeaders)
	if _, ok := reqCacheControl["no-cache"]; ok {
		return Transparent
	}
	if _, ok := respCacheControl["no-cache"]; ok {
		return Stale
	}
	if _, ok := reqCacheControl["only-if-cached"]; ok {
		return Fresh
	}

	date, err := getDate(respHeaders)
	if err != nil {
		return Stale
	}
	currentAge := clock.since(date)

	var lifetime time.Duration
	var zeroDuration time.Duration

	// If a response includes both an Expires header and a max-age directive,
	// the max-age directive overrides the Expires header, even if the Expires header is more restrictive.
	if maxAge, ok := respCacheControl["max-age"]; ok {
		lifetime, err = time.ParseDuration(maxAge + "s")
		if err != nil {
			lifetime = zeroDuration
		}
	} else {
		expiresHeader := respHeaders.Get("Expires")
		if expiresHeader != "" {
			var expires time.Time
			expires, err = time.Parse(time.RFC1123, expiresHeader)
			if err != nil {
				lifetime = zeroDuration
			} else {
				lifetime = expires.Sub(date)
			}
		}
	}

	if maxAge, ok := reqCacheControl["max-age"]; ok {
		// the client is willing to accept a response whose age is no greater than the specified time in seconds
		lifetime, err = time.ParseDuration(maxAge + "s")
		if err != nil {
			lifetime = zeroDuration
		}
	}
	if minFresh, ok := reqCacheControl["min-fresh"]; ok {
		//  the client wants a response that will still be fresh for at least the specified number of seconds.
		var minFreshDuration time.Duration
		minFreshDuration, err = time.ParseDuration(minFresh + "s")
		if err == nil {
			currentAge += minFreshDuration
		}
	}

	if maxStale, ok := reqCacheControl["max-stale"]; ok {
		// Indicates that the client is willing to accept a response that has exceeded its expiration time.
		// If max-stale is assigned a value, then the client is willing to accept a response that has exceeded
		// its expiration time by no more than the specified number of seconds.
		// If no value is assigned to max-stale, then the client is willing to accept a stale response of any age.
		//
		// Responses served only because of a max-stale value are supposed to have a Warning header added to them,
		// but that seems like a  hassle, and is it actually useful? If so, then there needs to be a different
		// return-value available here.
		if maxStale == "" {
			return Fresh
		}
		maxStaleDuration, err := time.ParseDuration(maxStale + "s")
		if err == nil {
			currentAge -= maxStaleDuration
		}
	}

	if lifetime > currentAge {
		return Fresh
	}

	return Stale
}

// canStaleOnError checks whether a stale cached response may be used when
// the origin server returns an error, per RFC 5861 (stale-if-error extension).
//
// The stale-if-error directive indicates that when an error is encountered
// (e.g., 5xx response, network failure), a stale cached response may be
// served instead of propagating the error to the client.
//
// This function returns true if either the cached response headers or the
// new request headers include a stale-if-error directive, AND the cached
// response's age is within the allowed staleness window.
//
// The directive can be specified as:
//   - "stale-if-error" (no value): Allows stale responses of any age
//   - "stale-if-error=N": Allows stale responses up to N seconds old
//
// Returns false if:
//   - Neither headers include stale-if-error
//   - The directive value is invalid
//   - The cached response's age exceeds the specified limit
//   - The response has no Date header (age cannot be computed)
//
// Reference: https://tools.ietf.org/html/rfc5861
func canStaleOnError(respHeaders, reqHeaders http.Header) bool {
	respCacheControl := parseCacheControl(respHeaders)
	reqCacheControl := parseCacheControl(reqHeaders)

	var err error
	lifetime := time.Duration(-1)

	if staleMaxAge, ok := respCacheControl["stale-if-error"]; ok {
		if staleMaxAge == "" {
			return true
		}
		lifetime, err = time.ParseDuration(staleMaxAge + "s")
		if err != nil {
			return false
		}
	}
	if staleMaxAge, ok := reqCacheControl["stale-if-error"]; ok {
		if staleMaxAge == "" {
			return true
		}
		lifetime, err = time.ParseDuration(staleMaxAge + "s")
		if err != nil {
			return false
		}
	}

	if lifetime >= 0 {
		date, err := getDate(respHeaders)
		if err != nil {
			return false
		}
		currentAge := clock.since(date)
		if lifetime > currentAge {
			return true
		}
	}

	return false
}

// getEndToEndHeaders returns a list of header names from respHeaders that are
// "end-to-end" headers (as opposed to "hop-by-hop" headers).
//
// Per RFC 7230 Section 6.1, hop-by-hop headers are meaningful only for a
// single transport-level connection and must not be retransmitted by proxies
// or stored in caches. End-to-end headers, conversely, are intended for the
// ultimate recipient and should be cached.
//
// This function filters out:
//   - Standard hop-by-hop headers: Connection, Keep-Alive, Proxy-Authenticate,
//     Proxy-Authorization, TE, Trailers, Transfer-Encoding, Upgrade
//   - Any additional headers listed in the Connection header value
//
// The returned headers are suitable for storage in a cache and for use when
// reconstructing a response from cache.
func getEndToEndHeaders(respHeaders http.Header) []string {
	// These headers are always hop-by-hop.
	hopByHopHeaders := map[string]struct{}{
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"Te":                  {},
		"Trailers":            {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
	}

	for extra := range strings.SplitSeq(respHeaders.Get("connection"), ",") {
		// any header listed in connection, if present, is also considered hop-by-hop
		if strings.Trim(extra, " ") != "" {
			hopByHopHeaders[http.CanonicalHeaderKey(extra)] = struct{}{}
		}
	}
	endToEndHeaders := []string{}
	for respHeader := range respHeaders {
		if _, ok := hopByHopHeaders[respHeader]; !ok {
			endToEndHeaders = append(endToEndHeaders, respHeader)
		}
	}
	return endToEndHeaders
}

// canStore determines whether a response may be stored in a cache based on
// the cache-control directives from the request and response.
//
// Per RFC 7234 Section 3, the "no-store" directive indicates that a cache
// MUST NOT store any part of the request or response. This function returns
// false if either the request or response includes "no-store", and true
// otherwise.
//
// Note: This function only checks the no-store directive. Other factors that
// might prevent caching (such as authorization headers or response status
// codes) are handled elsewhere.
func canStore(reqCacheControl, respCacheControl cacheControl) (canStore bool) {
	if _, ok := respCacheControl["no-store"]; ok {
		return false
	}
	if _, ok := reqCacheControl["no-store"]; ok {
		return false
	}
	return true
}

// newGatewayTimeoutResponse creates a synthetic HTTP 504 Gateway Timeout
// response for the given request.
//
// This is used when a request with "only-if-cached" cache-control directive
// is received but no valid cached response is available. Per RFC 7234
// Section 5.2.1.7, the cache should respond with 504 rather than forwarding
// the request to the origin server.
//
// Panics if the response cannot be parsed (which should never happen for
// the hardcoded response string).
func newGatewayTimeoutResponse(req *http.Request) *http.Response {
	var braw bytes.Buffer
	braw.WriteString("HTTP/1.1 504 Gateway Timeout\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(&braw), req)
	if err != nil {
		panic(err)
	}
	return resp
}

// cloneRequest returns a clone of the provided [http.Request].
//
// The clone is a shallow copy of the struct, with:
//   - The same context as the original
//   - A deep copy of the Header map (so modifications don't affect original)
//
// Other fields (Body, URL, etc.) are shared with the original request.
// This is used when we need to modify headers (e.g., adding conditional
// request headers like If-Modified-Since) without affecting the original.
//
// This function is adapted from goauth2: https://code.google.com/p/goauth2
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	if ctx := r.Context(); ctx != nil {
		r2 = r2.WithContext(ctx)
	}
	// deep copy of the Header
	r2.Header = make(http.Header)
	maps.Copy(r2.Header, r.Header)
	return r2
}

// cacheControl is a parsed representation of HTTP Cache-Control header
// directives. Keys are directive names (lowercase), values are directive
// values (or empty string for directives without values).
//
// Example: "max-age=3600, no-cache" parses to:
//
//	cacheControl{"max-age": "3600", "no-cache": ""}
type cacheControl map[string]string

// parseCacheControl parses the Cache-Control header from the given HTTP
// headers and returns a [cacheControl] map.
//
// The Cache-Control header consists of comma-separated directives, each
// optionally having a value (e.g., "max-age=3600, no-cache, private").
//
// Directives are parsed as:
//   - "directive" -> key="directive", value=""
//   - "directive=value" -> key="directive", value="value"
//
// Keys and values have surrounding whitespace trimmed. If no Cache-Control
// header is present, returns an empty map.
func parseCacheControl(headers http.Header) cacheControl {
	cc := cacheControl{}
	ccHeader := headers.Get("Cache-Control")
	for part := range strings.SplitSeq(ccHeader, ",") {
		part = strings.Trim(part, " ")
		if part == "" {
			continue
		}
		if strings.ContainsRune(part, '=') {
			keyval := strings.Split(part, "=")
			cc[strings.Trim(keyval[0], " ")] = strings.Trim(keyval[1], ",")
		} else {
			cc[part] = ""
		}
	}
	return cc
}

// headerAllCommaSepValues returns all comma-separated values (each
// with whitespace trimmed) for header name in headers. According to
// Section 4.2 of the HTTP/1.1 spec
// (http://www.w3.org/Protocols/rfc2616/rfc2616-sec4.html#sec4.2),
// values from multiple occurrences of a header should be concatenated, if
// the header's value is a comma-separated list.
func headerAllCommaSepValues(headers http.Header, name string) []string {
	var vals []string
	for _, val := range headers[http.CanonicalHeaderKey(name)] {
		fields := strings.Split(val, ",")
		for i, f := range fields {
			fields[i] = strings.TrimSpace(f)
		}
		vals = append(vals, fields...)
	}
	return vals
}

// varyMatches checks whether a cached response can be used for a new request
// based on the Vary header.
//
// The Vary header (RFC 7231 Section 7.1.4) lists request header fields that
// were used to select the representation. A cached response can only be used
// if the new request has the same values for all headers listed in Vary.
//
// When a response is cached, the values of varied headers from the original
// request are stored with an "X-Varied-" prefix (e.g., "X-Varied-Accept-Encoding").
// This function compares those stored values against the new request's headers.
//
// Returns true if all varied headers match (or if there is no Vary header),
// false if any varied header differs between the cached and new requests.
func varyMatches(cachedResp *http.Response, req *http.Request) bool {
	for _, header := range headerAllCommaSepValues(cachedResp.Header, "vary") {
		header = http.CanonicalHeaderKey(header)
		if header != "" && req.Header.Get(header) != cachedResp.Header.Get("X-Varied-"+header) {
			return false
		}
	}
	return true
}

// logResp logs information about an HTTP request/response cycle.
//
// This is a convenience function for consistent logging of HTTP operations
// throughout the downloader package. It logs:
//   - Response time (elapsed duration)
//   - Request method and URL (if req is non-nil)
//   - Error details (if err is non-nil, logs at Warn level)
//   - Response details (if no error, logs at Info level)
//
// If req is nil, logs "no_request: true" instead of method/URL.
func logResp(log *slog.Logger, req *http.Request, resp *http.Response, elapsed time.Duration, err error) {
	if req != nil {
		log = log.With("response_time", elapsed,
			lga.Method, req.Method,
			lga.URL, req.URL.String())
	} else {
		log = log.With("response_time", elapsed, "no_request", true)
	}

	if err != nil {
		log.Warn("HTTP request error", lga.Err, err)
		return
	}

	log.Info("HTTP request completed", lga.Resp, resp)
}
