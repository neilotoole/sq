package httpz_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/testh/tu"
)

// stubRoundTripper is a test http.RoundTripper.
type stubRoundTripper struct {
	resp   *http.Response
	err    error
	called bool
}

func (s *stubRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	s.called = true
	return s.resp, s.err
}

// panicRoundTripper is an http.RoundTripper that panics, simulating a buggy
// inner round-tripper.
type panicRoundTripper struct{}

func (panicRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	panic("inner round tripper panic")
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// ctxBoundBody is a response body that reads through ctx, returning ctx.Err()
// once ctx is canceled. It mimics a real transport's body, which reads through
// the request context, so a canceled context aborts the body read.
type ctxBoundBody struct {
	ctx context.Context
	r   io.Reader
}

func (b *ctxBoundBody) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.r.Read(p)
}

func (b *ctxBoundBody) Close() error { return nil }

// recordHandler is a slog.Handler that records emitted records for assertions.
type recordHandler struct{ records *[]slog.Record }

func (h recordHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h recordHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h recordHandler) WithGroup(string) slog.Handler            { return h }
func (h recordHandler) Handle(_ context.Context, r slog.Record) error {
	// A slog.Record is only valid for the duration of Handle; clone before
	// retaining it.
	*h.records = append(*h.records, r.Clone())
	return nil
}

func TestStatusText(t *testing.T) {
	require.Equal(t, "200 OK", httpz.StatusText(http.StatusOK))
	require.Equal(t, "404 Not Found", httpz.StatusText(http.StatusNotFound))
	require.Equal(t, "418 I'm a teapot", httpz.StatusText(http.StatusTeapot))
}

func TestNopTripFunc(t *testing.T) {
	stub := &stubRoundTripper{resp: &http.Response{StatusCode: http.StatusOK}}
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := httpz.NopTripFunc(stub, req)
	require.NoError(t, err)
	require.True(t, stub.called)
	require.Equal(t, 200, resp.StatusCode)
}

func TestFilename(t *testing.T) {
	mkResp := func(disp, urlStr string, withReq bool) *http.Response {
		resp := &http.Response{Header: http.Header{}}
		if disp != "" {
			resp.Header.Set("Content-Disposition", disp)
		}
		if withReq {
			u, err := url.Parse(urlStr)
			require.NoError(t, err)
			resp.Request = &http.Request{URL: u}
		}
		return resp
	}

	t.Run("content_disposition", func(t *testing.T) {
		resp := mkResp(`attachment; filename="report.csv"`, "https://x/ignored", true)
		require.Equal(t, "report.csv", httpz.Filename(resp))
	})

	t.Run("url_path_fallback", func(t *testing.T) {
		resp := mkResp("", "https://example.com/path/data.json", true)
		require.Equal(t, "data.json", httpz.Filename(resp))
	})

	t.Run("nil_response", func(t *testing.T) {
		require.Equal(t, "", httpz.Filename(nil))
	})

	t.Run("nil_header", func(t *testing.T) {
		require.Equal(t, "", httpz.Filename(&http.Response{}))
	})

	t.Run("nil_request_no_disposition", func(t *testing.T) {
		// Regression: must not panic when Request is nil and there's no
		// Content-Disposition (e.g. a response from ReadResponseHeader(r, nil)).
		require.NotPanics(t, func() {
			require.Equal(t, "", httpz.Filename(&http.Response{Header: http.Header{}}))
		})
	})

	t.Run("disposition_with_nil_request", func(t *testing.T) {
		// Content-Disposition path doesn't need Request.
		resp := mkResp(`attachment; filename="ok.txt"`, "", false)
		require.Equal(t, "ok.txt", httpz.Filename(resp))
	})
}

func TestResponseLogValue(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.NotPanics(t, func() { _ = httpz.ResponseLogValue(nil) })
	})

	t.Run("full", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
		require.NoError(t, err)
		resp := &http.Response{
			Request: req,
			Proto:   "HTTP/1.1",
			Status:  "200 OK",
			Header:  http.Header{},
		}
		resp.Header.Set("Content-Type", "text/plain") // single value
		resp.Header.Add("X-Multi", "a")               // multi value
		resp.Header.Add("X-Multi", "b")
		resp.Header.Set("Authorization", "Bearer super-secret-token") // sensitive
		v := httpz.ResponseLogValue(resp)
		s := v.String()
		require.Contains(t, s, "GET")
		require.Contains(t, s, "example.com/x")
		require.Contains(t, s, "200 OK")
		// Multi-value header logs all values (regression for the h.Get-only bug).
		require.Contains(t, s, "[a b]")
		// Credential-bearing header is redacted (using the repo's redaction token).
		require.Contains(t, s, "xxxxx")
		require.NotContains(t, s, "super-secret-token")
	})

	t.Run("request_with_nil_url", func(t *testing.T) {
		// Regression: Request set but URL nil must not panic.
		resp := &http.Response{Request: &http.Request{Method: http.MethodGet}, Header: http.Header{}}
		require.NotPanics(t, func() { _ = httpz.ResponseLogValue(resp) })
	})
}

func TestRequestLogValue(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.NotPanics(t, func() { _ = httpz.RequestLogValue(nil) })
	})

	t.Run("full", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, "https://example.com/api", nil)
		require.NoError(t, err)
		req.Header.Add("X-Multi", "a")
		req.Header.Add("X-Multi", "b")
		req.Header.Set("X-Single", "one")
		v := httpz.RequestLogValue(req)
		s := v.String()
		require.Contains(t, s, "POST")
		require.Contains(t, s, "/api")
	})

	t.Run("nil_url", func(t *testing.T) {
		// Regression: req.URL nil must not panic.
		require.NotPanics(t, func() { _ = httpz.RequestLogValue(&http.Request{Method: http.MethodGet}) })
	})

	t.Run("empty_path_uses_rawpath", func(t *testing.T) {
		// A URL with no path exercises the p == "" branch.
		req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
		require.NoError(t, err)
		require.NotPanics(t, func() { _ = httpz.RequestLogValue(req) })
	})
}

func TestOptResponseTimeout_nilResponseAndBody(t *testing.T) {
	// Call the TripFunc directly with a stub to exercise the guard for a
	// (resp == nil || resp.Body == nil) success result, which a real server
	// can't easily produce.
	tf := httpz.OptResponseTimeout(time.Second)
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	t.Run("nil_body", func(t *testing.T) {
		resp, err := tf(&stubRoundTripper{resp: &http.Response{Body: nil}}, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("nil_response", func(t *testing.T) {
		resp, err := tf(&stubRoundTripper{resp: nil}, req)
		require.NoError(t, err)
		require.Nil(t, resp)
	})
}

func TestOptRequestTimeout_directStub(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	t.Run("zero_is_noop", func(t *testing.T) {
		// timeout <= 0 returns NopTripFunc, which just passes through.
		tf := httpz.OptRequestTimeout(0)
		stub := &stubRoundTripper{resp: &http.Response{StatusCode: http.StatusOK}}
		resp, err := tf(stub, req)
		require.NoError(t, err)
		require.True(t, stub.called)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("nil_body_hits_default_branch", func(t *testing.T) {
		// A success result with a nil body exercises the final switch's default
		// branch (cancelFn is still called).
		tf := httpz.OptRequestTimeout(time.Second)
		resp, err := tf(&stubRoundTripper{resp: &http.Response{Body: nil}}, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("cancelled_parent_ctx_swaps_to_cause", func(t *testing.T) {
		// A pre-cancelled parent (with a distinct cause) drives the cause-swap
		// path: the inner tripper returns bare context.Canceled, which must be
		// swapped for the parent's cause. Using a sentinel cause makes the
		// assertion distinguishing (it would fail if the swap didn't happen).
		sentinel := errors.New("parent cause")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(sentinel)
		cReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
		require.NoError(t, err)

		tf := httpz.OptRequestTimeout(time.Hour)
		_, err = tf(&stubRoundTripper{err: context.Canceled}, cReq)
		require.ErrorIs(t, err, sentinel, "the bare context error must be swapped for the cause")
	})

	t.Run("panic_is_repropagated", func(t *testing.T) {
		// A panicking inner round-tripper must re-panic with the original value
		// (not be swallowed); the deferred cleanup releases the timer goroutine
		// and context first.
		tf := httpz.OptRequestTimeout(time.Hour)
		require.PanicsWithValue(t, "inner round tripper panic", func() {
			_, _ = tf(panicRoundTripper{}, req)
		})
	})
}

// TestOptRequestTimeout_bodyOutlivesHeaderTimeout is the regression test for
// #905: once the response headers have been received (RoundTrip returned
// successfully), the header timer must never cancel the context that the
// response body reads through, even if the timeout elapses while the body is
// still being read. It also asserts that no spurious "not received within
// timeout" warning is logged on the success path.
func TestOptRequestTimeout_bodyOutlivesHeaderTimeout(t *testing.T) {
	t.Parallel()

	const (
		timeout  = 40 * time.Millisecond
		respBody = "the body takes longer than the header timeout to read"
	)

	var records []slog.Record
	ctx := lg.NewContext(context.Background(), slog.New(recordHandler{&records}))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	// The round-tripper returns immediately (headers received well within the
	// timeout) with a body bound to the request context, exactly as a real
	// transport's body behaves.
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &ctxBoundBody{ctx: r.Context(), r: strings.NewReader(respBody)},
		}, nil
	})

	tf := httpz.OptRequestTimeout(timeout)
	resp, err := tf(rt, req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Wait well past the header timeout, simulating a slow body read. With the
	// pre-fix code the header timer could cancel the shared context here, making
	// the body read below fail with context.Canceled.
	time.Sleep(timeout * 4)

	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "body read must succeed: the header timer must not cancel after headers are received")
	require.Equal(t, respBody, string(got))
	require.NoError(t, resp.Body.Close())

	for _, r := range records {
		require.NotContains(t, r.Message, "not received within timeout",
			"no header-timeout warning must be logged when headers were received in time")
	}
}

// TestOptRequestTimeout_lateSuccessReportedAsTimeout verifies that when the
// header timer fires before RoundTrip returns, the result is reported
// consistently as a timeout error, even if RoundTrip then returns a response.
// Without this, the caller could receive a non-nil response whose body is dead
// because it reads through the already-canceled context.
func TestOptRequestTimeout_lateSuccessReportedAsTimeout(t *testing.T) {
	t.Parallel()

	const timeout = 40 * time.Millisecond

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	// The round-tripper returns a "successful" response, but only after the
	// header timeout has elapsed (so the timer fires first and cancels the
	// context). A real transport would return a context error here; this stub
	// returns success anyway to exercise the timer-won path deterministically.
	rt := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		time.Sleep(timeout * 3)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("too late")),
		}, nil
	})

	tf := httpz.OptRequestTimeout(timeout)
	resp, err := tf(rt, req)
	require.Error(t, err)
	require.Nil(t, resp, "a response must not be returned once the header timeout has fired")
	require.Contains(t, err.Error(), "http response header not received within")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// TestOptRequestTimeout_parentCancelNotReportedAsTimeout verifies that when the
// parent context is canceled around the same time the header timer fires, the
// failure is surfaced with the parent's cause (not a fabricated header timeout)
// and no misleading header-timeout warning is logged. This is the invariant the
// old code's `case <-ctx.Done()` select arm enforced.
func TestOptRequestTimeout_parentCancelNotReportedAsTimeout(t *testing.T) {
	t.Parallel()

	const timeout = 40 * time.Millisecond
	sentinel := errors.New("parent canceled")

	parent, cancel := context.WithCancelCause(context.Background())
	var records []slog.Record
	ctx := lg.NewContext(parent, slog.New(recordHandler{&records}))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	// Cancel the parent up front; the stub ignores the context and returns a
	// "success" only after the header timeout has elapsed, so the timer fires
	// while the parent is already canceled.
	cancel(sentinel)
	rt := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		time.Sleep(timeout * 3)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("x")),
		}, nil
	})

	tf := httpz.OptRequestTimeout(timeout)
	resp, err := tf(rt, req)
	require.Error(t, err)
	require.Nil(t, resp)
	require.ErrorIs(t, err, sentinel, "the parent's cause must be surfaced, not a fabricated timeout")
	for _, r := range records {
		require.NotContains(t, r.Message, "not received within timeout",
			"a parent cancellation must not be logged as a header timeout")
	}
}

func TestOptResponseTimeout_logsOnCloseAfterTimeout(t *testing.T) {
	// When the body is closed after the response timeout has already elapsed,
	// the ReadCloserNotifier callback logs (the errors.Is(cause, timeoutErr)
	// branch). Capture the log to verify that branch actually fires.
	var records []slog.Record
	ctx := lg.NewContext(context.Background(), slog.New(recordHandler{&records}))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	// Short timeout with a generous sleep margin so the timeout has reliably
	// elapsed (and the context cause is the timeout error) before Close, even
	// under loaded CI.
	tf := httpz.OptResponseTimeout(20 * time.Millisecond)
	stub := &stubRoundTripper{resp: &http.Response{Body: io.NopCloser(strings.NewReader("x"))}}
	resp, err := tf(stub, req)
	require.NoError(t, err)

	time.Sleep(250 * time.Millisecond)
	require.NoError(t, resp.Body.Close())

	var logged bool
	for _, r := range records {
		if strings.Contains(r.Message, "HTTP request not completed within timeout") {
			logged = true
		}
	}
	require.True(t, logged, "closing the body after the timeout must log the warning")
}

func TestReadResponseHeader(t *testing.T) {
	mk := func(raw string) (*http.Response, error) {
		return httpz.ReadResponseHeader(bufio.NewReader(strings.NewReader(raw)), nil)
	}

	t.Run("valid_with_pragma", func(t *testing.T) {
		resp, err := mk("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nPragma: no-cache\r\n\r\n")
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		require.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
		// fixPragmaCacheControl: Pragma: no-cache adds Cache-Control: no-cache.
		require.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
		require.Nil(t, resp.Body)
	})

	t.Run("malformed_status_line", func(t *testing.T) {
		_, err := mk("garbage\r\n\r\n")
		require.Error(t, err)
		require.Contains(t, err.Error(), "malformed HTTP response")
	})

	t.Run("malformed_status_code", func(t *testing.T) {
		_, err := mk("HTTP/1.1 20 OK\r\n\r\n")
		require.Error(t, err)
		require.Contains(t, err.Error(), "malformed HTTP status code")
	})

	t.Run("non_numeric_status_code", func(t *testing.T) {
		_, err := mk("HTTP/1.1 abc OK\r\n\r\n")
		require.Error(t, err)
		require.Contains(t, err.Error(), "malformed HTTP status code")
	})

	t.Run("malformed_version", func(t *testing.T) {
		_, err := mk("BADPROTO 200 OK\r\n\r\n")
		require.Error(t, err)
		require.Contains(t, err.Error(), "malformed HTTP version")
	})

	t.Run("eof", func(t *testing.T) {
		_, err := mk("")
		require.Error(t, err)
	})

	t.Run("eof_after_status_line", func(t *testing.T) {
		// Status line but truncated headers.
		_, err := mk("HTTP/1.1 200 OK\r\n")
		require.Error(t, err)
	})
}

func TestNewDefaultClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := httpz.NewDefaultClient()
	require.NotNil(t, c)
	require.Zero(t, c.Timeout)

	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "ok", tu.ReadToString(t, resp.Body))
}

func TestOptInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("secure"))
	}))
	t.Cleanup(srv.Close)

	// Without skip-verify, the self-signed cert is rejected.
	_, err := httpz.NewClient().Get(srv.URL)
	require.Error(t, err)

	// With skip-verify, the request succeeds.
	c := httpz.NewClient(httpz.OptInsecureSkipVerify(true))
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "secure", tu.ReadToString(t, resp.Body))
}

func TestOptUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
	}))
	t.Cleanup(srv.Close)

	c := httpz.NewClient(httpz.OptUserAgent("my-agent/1.0"))
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, "my-agent/1.0", gotUA)
}

func TestOptResponseTimeout_success(t *testing.T) {
	const body = "hello"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	// A generous timeout: the request completes well within it, exercising the
	// success path that wraps resp.Body to cancel the context on close.
	c := httpz.NewClient(httpz.OptResponseTimeout(30 * time.Second))
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, body, tu.ReadToString(t, resp.Body))
	require.NoError(t, resp.Body.Close())
}

func TestOptResponseTimeout_zeroIsNoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	c := httpz.NewClient(httpz.OptResponseTimeout(0))
	resp, err := c.Get(srv.URL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestOptRequestDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	t.Run("delays_then_succeeds", func(t *testing.T) {
		c := httpz.NewClient(httpz.OptRequestDelay(50 * time.Millisecond))
		start := time.Now()
		resp, err := c.Get(srv.URL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		require.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)
	})

	t.Run("cancelled_context_aborts_delay_direct", func(t *testing.T) {
		// Direct stub so we unambiguously exercise OptRequestDelay's ctx.Done
		// branch: a pre-cancelled context must abort the delay, return the
		// cause, and never call next (rather than the stdlib client
		// short-circuiting before the delay TripFunc runs).
		ctx, cancel := context.WithCancelCause(context.Background())
		sentinel := errors.New("delay cancel cause")
		cancel(sentinel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
		require.NoError(t, err)

		tf := httpz.OptRequestDelay(time.Hour)
		stub := &stubRoundTripper{resp: &http.Response{StatusCode: http.StatusOK}}
		_, err = tf(stub, req)
		require.ErrorIs(t, err, sentinel)
		require.False(t, stub.called, "delay must abort before calling next")
	})

	t.Run("cancelled_context_via_client", func(t *testing.T) {
		// Through NewClient (exercises the outermost contextCause swap).
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		require.NoError(t, err)

		c := httpz.NewClient(httpz.OptRequestDelay(time.Hour))
		_, err = c.Do(req)
		require.Error(t, err)
	})

	t.Run("zero_is_noop", func(t *testing.T) {
		c := httpz.NewClient(httpz.OptRequestDelay(0))
		resp, err := c.Get(srv.URL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
