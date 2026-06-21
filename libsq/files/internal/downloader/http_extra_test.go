// This file contains additional white-box tests for the HTTP caching helpers
// in http.go, targeting parse-error and edge-case branches not exercised by
// the existing tests in http_test.go.

package downloader

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg/lgt"
)

// TestGetFreshness_Extra covers the parse-error and fallback branches of
// getFreshness that the main TestGetFreshness table doesn't reach:
//
//   - response max-age value that fails to parse (lifetime -> 0).
//   - request max-age value that fails to parse (lifetime -> 0).
//   - request min-fresh value that fails to parse (currentAge unchanged).
//   - request max-stale value that fails to parse (currentAge unchanged).
//   - Expires header that fails to parse (lifetime -> 0).
//
// It reuses the mockClock pattern from http_test.go (same package), saving and
// restoring the package-level clock.
func TestGetFreshness_Extra(t *testing.T) {
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
			// Unparseable response max-age -> lifetime 0 -> stale (0 > 0 false).
			name:        "resp_max_age_invalid",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=notanumber"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Minute,
			want:        Stale,
		},
		{
			// Unparseable request max-age -> lifetime 0 -> stale.
			name:        "req_max_age_invalid",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=3600"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"max-age=bogus"}},
			elapsed:     time.Minute,
			want:        Stale,
		},
		{
			// Unparseable min-fresh -> currentAge unchanged; lifetime 1h > 30min -> fresh.
			name:        "min_fresh_invalid_ignored",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=3600"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"min-fresh=notnum"}},
			elapsed:     time.Minute * 30,
			want:        Fresh,
		},
		{
			// Unparseable max-stale (with a value) -> currentAge unchanged; still stale.
			name:        "max_stale_invalid_ignored",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"max-age=1800"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"max-stale=xyz"}},
			elapsed:     time.Hour,
			want:        Stale,
		},
		{
			// Unparseable Expires header -> lifetime 0 -> stale.
			name:        "expires_invalid",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Expires": []string{"not-a-date"}},
			reqHeaders:  http.Header{},
			elapsed:     time.Minute,
			want:        Stale,
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

// TestCanStaleOnError_Extra covers the request-side branches of canStaleOnError
// that the main TestCanStaleOnError table doesn't reach:
//
//   - request stale-if-error with an empty value -> early return true.
//   - request stale-if-error with an invalid value -> return false.
func TestCanStaleOnError_Extra(t *testing.T) {
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
			// Request stale-if-error present with empty value -> true (req-side
			// empty-value early return).
			name:        "req_stale_if_error_empty_after_resp_value",
			respHeaders: http.Header{"Date": []string{dateHeader}, "Cache-Control": []string{"stale-if-error=3600"}},
			reqHeaders:  http.Header{"Cache-Control": []string{"stale-if-error"}},
			elapsed:     time.Minute,
			want:        true,
		},
		{
			// Request stale-if-error with an invalid (unparseable) value -> false
			// (req-side ParseDuration error branch).
			name:        "req_stale_if_error_invalid",
			respHeaders: http.Header{"Date": []string{dateHeader}},
			reqHeaders:  http.Header{"Cache-Control": []string{"stale-if-error=notnum"}},
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

// TestLogResp covers the logResp branches:
//   - req == nil -> "no_request" branch.
//   - err != nil -> Warn branch.
//   - happy path -> Info branch.
func TestLogResp(t *testing.T) {
	log := lgt.New(t)

	// req nil, err nil -> no_request + Info.
	logResp(log, nil, nil, time.Second, nil)

	// req non-nil, err non-nil -> Warn branch.
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	require.NoError(t, err)
	logResp(log, req, nil, time.Second, errors.New("boom"))

	// req nil, err non-nil -> no_request + Warn.
	logResp(log, nil, nil, time.Second, errors.New("boom2"))
}
