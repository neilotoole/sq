// This file contains internal tests for the HTTP caching helper functions
// in http.go. It uses package downloader (not downloader_test) to access
// unexported functions like getDate, parseCacheControl, canStore, etc.
//
// These tests verify the RFC 7234 (HTTP Caching) and RFC 5861 (stale-if-error)
// compliance of the cache control logic.

package downloader

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestGetDate tests the [getDate] function, which parses the HTTP Date header
// from response headers.
//
// The Date header is essential for computing cache age per RFC 7234. This test
// verifies:
//   - Valid RFC 1123 formatted dates are parsed correctly
//   - Missing Date header returns [errNoDateHeader]
//   - Empty Date header returns [errNoDateHeader]
//   - Malformed date strings return a parse error
func TestGetDate(t *testing.T) {
	testCases := []struct {
		name      string
		headers   http.Header
		wantErr   bool
		wantTime  time.Time
		errString string
	}{
		{
			name:    "valid_date",
			headers: http.Header{"Date": []string{"Mon, 02 Jan 2006 15:04:05 GMT"}},
			wantErr: false,
			// time.RFC1123 = "Mon, 02 Jan 2006 15:04:05 MST"
			wantTime: time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name:      "no_date_header",
			headers:   http.Header{},
			wantErr:   true,
			errString: "no Date header",
		},
		{
			name:      "empty_date_header",
			headers:   http.Header{"Date": []string{""}},
			wantErr:   true,
			errString: "no Date header",
		},
		{
			name:    "invalid_date_format",
			headers: http.Header{"Date": []string{"2006-01-02"}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getDate(tc.headers)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errString != "" {
					require.Contains(t, err.Error(), tc.errString)
				}
				return
			}
			require.NoError(t, err)
			require.True(t, tc.wantTime.Equal(got), "want %v, got %v", tc.wantTime, got)
		})
	}
}

// TestParseCacheControl tests the [parseCacheControl] function, which parses
// the Cache-Control header into a map of directive names to values.
//
// This test verifies parsing of various Cache-Control formats:
//   - Empty headers return empty map
//   - Directives without values (no-cache, no-store)
//   - Directives with values (max-age=3600)
//   - Multiple comma-separated directives
//   - Whitespace handling
//   - RFC 5861 extension directive (stale-if-error)
func TestParseCacheControl(t *testing.T) {
	testCases := []struct {
		name    string
		headers http.Header
		want    cacheControl
	}{
		{
			name:    "empty",
			headers: http.Header{},
			want:    cacheControl{},
		},
		{
			name:    "no_cache",
			headers: http.Header{"Cache-Control": []string{"no-cache"}},
			want:    cacheControl{"no-cache": ""},
		},
		{
			name:    "no_store",
			headers: http.Header{"Cache-Control": []string{"no-store"}},
			want:    cacheControl{"no-store": ""},
		},
		{
			name:    "max_age",
			headers: http.Header{"Cache-Control": []string{"max-age=3600"}},
			want:    cacheControl{"max-age": "3600"},
		},
		{
			name:    "multiple_directives",
			headers: http.Header{"Cache-Control": []string{"no-cache, max-age=0"}},
			want:    cacheControl{"no-cache": "", "max-age": "0"},
		},
		{
			name:    "with_spaces",
			headers: http.Header{"Cache-Control": []string{" no-cache , max-age=100 "}},
			want:    cacheControl{"no-cache": "", "max-age": "100"},
		},
		{
			name:    "stale_if_error",
			headers: http.Header{"Cache-Control": []string{"stale-if-error=86400"}},
			want:    cacheControl{"stale-if-error": "86400"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCacheControl(tc.headers)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCanStore tests the [canStore] function, which determines whether a
// response may be cached based on the no-store directive.
//
// Per RFC 7234 Section 3, the no-store directive indicates that a cache
// MUST NOT store any part of the request or response. This test verifies:
//   - Empty cache-control allows storage
//   - Response no-store prevents storage
//   - Request no-store prevents storage
//   - Both no-store prevents storage
//   - Other directives (max-age, no-cache) do not prevent storage
func TestCanStore(t *testing.T) {
	testCases := []struct {
		name     string
		reqCC    cacheControl
		respCC   cacheControl
		canStore bool
	}{
		{
			name:     "both_empty",
			reqCC:    cacheControl{},
			respCC:   cacheControl{},
			canStore: true,
		},
		{
			name:     "resp_no_store",
			reqCC:    cacheControl{},
			respCC:   cacheControl{"no-store": ""},
			canStore: false,
		},
		{
			name:     "req_no_store",
			reqCC:    cacheControl{"no-store": ""},
			respCC:   cacheControl{},
			canStore: false,
		},
		{
			name:     "both_no_store",
			reqCC:    cacheControl{"no-store": ""},
			respCC:   cacheControl{"no-store": ""},
			canStore: false,
		},
		{
			name:     "other_directives",
			reqCC:    cacheControl{"max-age": "3600"},
			respCC:   cacheControl{"no-cache": ""},
			canStore: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := canStore(tc.reqCC, tc.respCC)
			require.Equal(t, tc.canStore, got)
		})
	}
}

// TestHeaderAllCommaSepValues tests the [headerAllCommaSepValues] function,
// which extracts all comma-separated values from an HTTP header.
//
// Per RFC 7230 Section 3.2.2, multiple header field instances with the same
// name are equivalent to a single instance with comma-separated values.
// This test verifies:
//   - Empty headers return nil
//   - Single value headers
//   - Comma-separated values in a single header
//   - Multiple header instances (same name, different values)
//   - Whitespace trimming around values
//   - Case-insensitive header name lookup
func TestHeaderAllCommaSepValues(t *testing.T) {
	testCases := []struct {
		name    string
		headers http.Header
		header  string
		want    []string
	}{
		{
			name:    "empty",
			headers: http.Header{},
			header:  "Vary",
			want:    nil,
		},
		{
			name:    "single_value",
			headers: http.Header{"Vary": []string{"Accept"}},
			header:  "Vary",
			want:    []string{"Accept"},
		},
		{
			name:    "comma_separated",
			headers: http.Header{"Vary": []string{"Accept, Accept-Encoding"}},
			header:  "Vary",
			want:    []string{"Accept", "Accept-Encoding"},
		},
		{
			name:    "multiple_headers",
			headers: http.Header{"Vary": []string{"Accept", "Accept-Language"}},
			header:  "Vary",
			want:    []string{"Accept", "Accept-Language"},
		},
		{
			name:    "with_spaces",
			headers: http.Header{"Vary": []string{" Accept , Accept-Encoding "}},
			header:  "Vary",
			want:    []string{"Accept", "Accept-Encoding"},
		},
		{
			name:    "case_insensitive_lookup",
			headers: http.Header{"Vary": []string{"Accept"}},
			header:  "vary",
			want:    []string{"Accept"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := headerAllCommaSepValues(tc.headers, tc.header)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestGetEndToEndHeaders tests the [getEndToEndHeaders] function, which
// filters HTTP headers to return only end-to-end headers (excluding hop-by-hop).
//
// Per RFC 7230 Section 6.1, hop-by-hop headers are meaningful only for a
// single connection and must not be cached. This test verifies:
//   - Empty headers return empty result
//   - Standard hop-by-hop headers are filtered: Connection, Keep-Alive,
//     Proxy-Authenticate, Proxy-Authorization, TE, Trailers, Transfer-Encoding,
//     Upgrade
//   - Headers listed in the Connection header value are also filtered
//   - Content headers (Content-Type, etc.) are preserved as end-to-end
func TestGetEndToEndHeaders(t *testing.T) {
	testCases := []struct {
		name        string
		headers     http.Header
		wantInclude []string
		wantExclude []string
	}{
		{
			name:        "empty",
			headers:     http.Header{},
			wantInclude: nil,
			wantExclude: nil,
		},
		{
			name: "filters_hop_by_hop",
			headers: http.Header{
				"Content-Type": []string{"text/plain"},
				"Connection":   []string{"keep-alive"},
				"Keep-Alive":   []string{"timeout=5"},
			},
			wantInclude: []string{"Content-Type"},
			wantExclude: []string{"Connection", "Keep-Alive"},
		},
		{
			name: "filters_all_hop_by_hop",
			headers: http.Header{
				"Content-Type":        []string{"text/plain"},
				"Connection":          []string{""},
				"Keep-Alive":          []string{""},
				"Proxy-Authenticate":  []string{""},
				"Proxy-Authorization": []string{""},
				"Te":                  []string{""},
				"Trailers":            []string{""},
				"Transfer-Encoding":   []string{""},
				"Upgrade":             []string{""},
			},
			wantInclude: []string{"Content-Type"},
			wantExclude: []string{
				"Connection", "Keep-Alive", "Proxy-Authenticate",
				"Proxy-Authorization", "Te", "Trailers", "Transfer-Encoding", "Upgrade",
			},
		},
		{
			name: "connection_listed_headers",
			headers: http.Header{
				"Content-Type":    []string{"text/plain"},
				"Connection":      []string{"X-Custom-Header"},
				"X-Custom-Header": []string{"value"},
			},
			wantInclude: []string{"Content-Type"},
			wantExclude: []string{"Connection", "X-Custom-Header"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getEndToEndHeaders(tc.headers)
			for _, h := range tc.wantInclude {
				require.Contains(t, got, h, "should include %s", h)
			}
			for _, h := range tc.wantExclude {
				require.NotContains(t, got, h, "should exclude %s", h)
			}
		})
	}
}

// mockClock is a mock implementation of the [timer] interface for testing
// time-dependent cache functions like [getFreshness] and [canStaleOnError].
//
// By replacing the package-level [clock] variable with a mockClock, tests
// can control the "elapsed time since response" without waiting for real
// time to pass. This enables deterministic testing of cache freshness logic.
//
// Usage:
//
//	origClock := clock
//	t.Cleanup(func() { clock = origClock })
//	clock = &mockClock{elapsed: time.Hour}
type mockClock struct {
	// elapsed is the fixed duration returned by since(), representing
	// how much time has "passed" since the response was generated.
	elapsed time.Duration
}

// since returns the pre-configured elapsed duration, ignoring the input time.
// This allows tests to simulate any cache age without actual time passage.
func (m *mockClock) since(_ time.Time) time.Duration {
	return m.elapsed
}

// TestGetFreshness comprehensively tests the [getFreshness] function, which
// determines whether a cached response is Fresh, Stale, or Transparent based
// on HTTP cache-control semantics.
//
// This test uses [mockClock] to control time-based freshness calculations.
// It verifies RFC 7234 cache freshness rules:
//
// Request directives:
//   - no-cache: Returns [Transparent] (bypass cache)
//   - only-if-cached: Returns [Fresh] (use cache regardless of age)
//   - max-age: Limits acceptable response age
//   - min-fresh: Requires minimum remaining freshness
//   - max-stale: Accepts stale responses within tolerance
//
// Response directives:
//   - no-cache: Returns [Stale] (always revalidate)
//   - max-age: Defines freshness lifetime
//   - Expires header: Fallback when max-age absent
//
// Edge cases:
//   - Missing Date header returns [Stale]
//   - Request max-age overrides response max-age
func TestGetFreshness(t *testing.T) {
	// Save original clock and restore after test
	origClock := clock
	t.Cleanup(func() { clock = origClock })

	now := time.Now().UTC()
	dateHeader := now.Format(time.RFC1123)

	testCases := []struct {
		name        string
		respHeaders http.Header
		reqHeaders  http.Header
		elapsed     time.Duration
		want        State
	}{
		{
			name:        "req_no_cache_returns_transparent",
			respHeaders: http.Header{"Date": []string{dateHeader}},
			reqHeaders:  http.Header{"Cache-Control": []string{"no-cache"}},
			elapsed:     0,
			want:        Transparent,
		},
		{
			name:        "resp_no_cache_returns_stale",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"no-cache"}},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        Stale,
		},
		{
			name:        "req_only_if_cached_returns_fresh",
			respHeaders: http.Header{"Date": []string{dateHeader}},
			reqHeaders:  http.Header{"Cache-Control": []string{"only-if-cached"}},
			elapsed:     0,
			want:        Fresh,
		},
		{
			name:        "no_date_header_returns_stale",
			respHeaders: http.Header{},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        Stale,
		},
		{
			name:        "max_age_fresh",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=3600"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Minute * 30, // 30 min < 1 hour
			want:        Fresh,
		},
		{
			name:        "max_age_stale",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=3600"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Hour * 2, // 2 hours > 1 hour
			want:        Stale,
		},
		{
			name:        "expires_header_fresh",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Expires": []string{now.Add(time.Hour).Format(time.RFC1123)}},
			reqHeaders:  http.Header{},
			elapsed:     time.Minute * 30,
			want:        Fresh,
		},
		{
			name:        "expires_header_stale",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Expires": []string{now.Add(time.Hour).Format(time.RFC1123)}},
			reqHeaders:  http.Header{},
			elapsed:     time.Hour * 2,
			want:        Stale,
		},
		{
			name:        "req_max_age_overrides_resp",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=7200"}}, // 2 hours
			reqHeaders:  http.Header{"Cache-Control": []string{"max-age=1800"}},                               // 30 min
			elapsed:     time.Hour,                                                                            // 1 hour > 30 min
			want:        Stale,
		},
		{
			// 1h lifetime, wants 30min freshness, 45min elapsed + 30min = 75min > 60min.
			name:        "min_fresh_makes_stale",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=3600"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"min-fresh=1800"}},
			elapsed:     time.Minute * 45,
			want:        Stale,
		},
		{
			// 30min max-age, 1h elapsed, but max-stale (empty) accepts any stale.
			name:        "max_stale_empty_returns_fresh",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=1800"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"max-stale"}},
			elapsed:     time.Hour,
			want:        Fresh,
		},
		{
			// 30min max-age, 1h elapsed (30min stale), max-stale=1h tolerance.
			name:        "max_stale_with_value",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=1800"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"max-stale=3600"}},
			elapsed:     time.Hour,
			want:        Fresh,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clock = &mockClock{elapsed: tc.elapsed}
			got := getFreshness(tc.respHeaders, tc.reqHeaders)
			require.Equal(t, tc.want, got, "getFreshness returned %s, want %s", got, tc.want)
		})
	}
}

// TestCanStaleOnError tests the [canStaleOnError] function, which implements
// RFC 5861 (HTTP Cache-Control Extensions for Stale Content).
//
// The stale-if-error directive allows serving stale cached content when the
// origin server returns an error. This test uses [mockClock] to control time
// and verifies:
//
// Directive presence:
//   - No stale-if-error: Returns false
//   - Empty stale-if-error value: Allows any stale age (returns true)
//   - stale-if-error in response headers
//   - stale-if-error in request headers
//
// Time-based validation:
//   - Within stale-if-error lifetime: Returns true
//   - Exceeded stale-if-error lifetime: Returns false
//
// Edge cases:
//   - Missing Date header: Returns false (can't compute age)
//   - Invalid stale-if-error value: Returns false
func TestCanStaleOnError(t *testing.T) {
	// Save original clock and restore after test
	origClock := clock
	t.Cleanup(func() { clock = origClock })

	now := time.Now().UTC()
	dateHeader := now.Format(time.RFC1123)

	testCases := []struct {
		name        string
		respHeaders http.Header
		reqHeaders  http.Header
		elapsed     time.Duration
		want        bool
	}{
		{
			name:        "no_stale_if_error",
			respHeaders: http.Header{"Date": []string{dateHeader}},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        false,
		},
		{
			name:        "resp_stale_if_error_empty_value",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"stale-if-error"}},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        true,
		},
		{
			name:        "req_stale_if_error_empty_value",
			respHeaders: http.Header{"Date": []string{dateHeader}},
			reqHeaders:  http.Header{"Cache-Control": []string{"stale-if-error"}},
			elapsed:     0,
			want:        true,
		},
		{
			name:        "resp_stale_if_error_within_lifetime",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"stale-if-error=3600"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Minute * 30,
			want:        true,
		},
		{
			name:        "resp_stale_if_error_expired",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"stale-if-error=1800"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Hour,
			want:        false,
		},
		{
			name:        "no_date_header",
			respHeaders: http.Header{"Cache-Control": []string{"stale-if-error=3600"}},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        false,
		},
		{
			name:        "invalid_stale_if_error_value",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"stale-if-error=invalid"}},
			reqHeaders:  http.Header{},
			elapsed:     0,
			want:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clock = &mockClock{elapsed: tc.elapsed}
			got := canStaleOnError(tc.respHeaders, tc.reqHeaders)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestVaryMatches tests the [varyMatches] function, which determines whether
// a cached response can be used for a new request based on the Vary header.
//
// The Vary header (RFC 7231 Section 7.1.4) lists request headers that the
// server used to select the response representation. A cached response can
// only be reused if the new request has matching values for all varied headers.
//
// When caching, the original request's varied header values are stored with
// an "X-Varied-" prefix. This test verifies:
//
// Basic cases:
//   - No Vary header: Always matches
//   - Single Vary header that matches
//   - Single Vary header that doesn't match
//
// Multiple headers:
//   - Multiple varied headers that all match
//   - Multiple varied headers where one doesn't match
func TestVaryMatches(t *testing.T) {
	testCases := []struct {
		name       string
		cachedResp *http.Response
		req        *http.Request
		want       bool
	}{
		{
			name: "no_vary_header",
			cachedResp: &http.Response{
				Header: http.Header{},
			},
			req: &http.Request{
				Header: http.Header{"Accept": []string{"text/html"}},
			},
			want: true,
		},
		{
			name: "vary_matches",
			cachedResp: &http.Response{
				Header: http.Header{
					"Vary":            []string{"Accept"},
					"X-Varied-Accept": []string{"text/html"},
				},
			},
			req: &http.Request{
				Header: http.Header{"Accept": []string{"text/html"}},
			},
			want: true,
		},
		{
			name: "vary_does_not_match",
			cachedResp: &http.Response{
				Header: http.Header{
					"Vary":            []string{"Accept"},
					"X-Varied-Accept": []string{"text/html"},
				},
			},
			req: &http.Request{
				Header: http.Header{"Accept": []string{"application/json"}},
			},
			want: false,
		},
		{
			name: "vary_multiple_headers_match",
			cachedResp: &http.Response{
				Header: http.Header{
					"Vary":                     []string{"Accept, Accept-Encoding"},
					"X-Varied-Accept":          []string{"text/html"},
					"X-Varied-Accept-Encoding": []string{"gzip"},
				},
			},
			req: &http.Request{
				Header: http.Header{
					"Accept":          []string{"text/html"},
					"Accept-Encoding": []string{"gzip"},
				},
			},
			want: true,
		},
		{
			name: "vary_one_header_mismatch",
			cachedResp: &http.Response{
				Header: http.Header{
					"Vary":                     []string{"Accept, Accept-Encoding"},
					"X-Varied-Accept":          []string{"text/html"},
					"X-Varied-Accept-Encoding": []string{"gzip"},
				},
			},
			req: &http.Request{
				Header: http.Header{
					"Accept":          []string{"text/html"},
					"Accept-Encoding": []string{"br"},
				},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := varyMatches(tc.cachedResp, tc.req)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestNewGatewayTimeoutResponse tests the [newGatewayTimeoutResponse] function,
// which creates a synthetic HTTP 504 Gateway Timeout response.
//
// This response is used when a request includes "only-if-cached" but no valid
// cached response exists. Per RFC 7234 Section 5.2.1.7, the cache should
// return 504 rather than forwarding to the origin server.
//
// The test verifies:
//   - A valid response object is returned (not nil)
//   - The status code is 504 Gateway Timeout
func TestNewGatewayTimeoutResponse(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)

	resp := newGatewayTimeoutResponse(req)
	require.NotNil(t, resp)
	require.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
}

// TestCloneRequest tests the [cloneRequest] function, which creates a shallow
// copy of an [http.Request] with a deep copy of the Header map.
//
// This function is used when the Downloader needs to modify headers (e.g.,
// adding If-Modified-Since for conditional requests) without affecting the
// original request.
//
// The test verifies:
//   - The cloned request is a different object (not same pointer)
//   - Headers are initially copied correctly
//   - Modifying cloned headers does NOT affect original (deep copy)
//   - URL and Method are preserved (shallow copy is sufficient)
func TestCloneRequest(t *testing.T) {
	original, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	original.Header.Set("X-Custom", "value")

	cloned := cloneRequest(original)

	// Verify it's a different object
	require.NotSame(t, original, cloned)

	// Verify headers are copied
	require.Equal(t, original.Header.Get("X-Custom"), cloned.Header.Get("X-Custom"))

	// Verify modifying cloned headers doesn't affect original
	cloned.Header.Set("X-Custom", "modified")
	require.Equal(t, "value", original.Header.Get("X-Custom"))
	require.Equal(t, "modified", cloned.Header.Get("X-Custom"))

	// Verify URL and method are same
	require.Equal(t, original.URL.String(), cloned.URL.String())
	require.Equal(t, original.Method, cloned.Method)
}
