package httpz

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Opt is an option that can be passed to NewClient to
// configure the client.
type Opt interface {
	apply(*http.Transport)
}

var _ Opt = (*OptInsecureSkipVerify)(nil)

// OptInsecureSkipVerify is an Opt that can be passed to NewClient that,
// when true, disables TLS verification.
type OptInsecureSkipVerify bool

func (b OptInsecureSkipVerify) apply(tr *http.Transport) {
	// Guard against a nil config: NewClient applies the min-TLS opt first (which
	// creates the config), but don't assume that ordering here. The literal
	// MinVersion is a safe default floor (also satisfies gosec G402); it's
	// raised/clamped by the min-TLS opt.
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tr.TLSClientConfig.InsecureSkipVerify = bool(b)
}

var _ Opt = (*minTLSVersion)(nil)

type minTLSVersion uint16

func (v minTLSVersion) apply(tr *http.Transport) {
	if tr.TLSClientConfig == nil {
		// The literal MinVersion (a constant >= TLS 1.2) satisfies gosec G402;
		// the requested version is applied via the field assignment below, which
		// gosec does not flag.
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		// Preserve any settings already on the config (RootCAs, ServerName,
		// etc.) by cloning rather than replacing it.
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	}
	tr.TLSClientConfig.MinVersion = uint16(v)
}

// DefaultTLSVersion is the default minimum TLS version, as used by
// NewDefaultClient. It matches the Go standard library client default
// (TLS 1.2); TLS 1.0 and 1.1 are deprecated by RFC 8996.
var DefaultTLSVersion = minTLSVersion(tls.VersionTLS12)

// OptUserAgent is passed to NewClient to set the User-Agent header.
func OptUserAgent(ua string) TripFunc {
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", ua)
		return next.RoundTrip(req)
	}
}

// DefaultUserAgent is the default User-Agent header value,
// as used by NewDefaultClient.
var DefaultUserAgent = OptUserAgent(buildinfo.Get().UserAgent())

// OptResponseTimeout is passed to NewClient to set the total request timeout,
// including reading the body. This is basically the same as a traditional
// request timeout via context.WithTimeout. If timeout is zero, this is no-op.
//
// Contrast with OptRequestTimeout.
func OptResponseTimeout(timeout time.Duration) TripFunc {
	if timeout <= 0 {
		return NopTripFunc
	}

	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		timeoutErr := errz.Wrapf(context.DeadlineExceeded,
			"http request not completed within %s timeout", timeout)
		ctx, cancelFn := context.WithTimeoutCause(req.Context(), timeout, timeoutErr)

		resp, err := next.RoundTrip(req.WithContext(ctx))
		if err == nil {
			if resp == nil || resp.Body == nil {
				// Shouldn't happen, but just in case.
				cancelFn()
			} else {
				// Wrap resp.Body with a ReadCloserNotifier, so that cancelFn
				// is called when the body is closed.
				resp.Body = ioz.ReadCloserNotifier(resp.Body, func(_ error) {
					if errors.Is(context.Cause(ctx), timeoutErr) {
						lg.FromContext(ctx).Warn("HTTP request not completed within timeout",
							lga.Timeout, timeout, lga.URL, req.URL.String())
					}

					cancelFn()
				})
			}
			return resp, nil
		}

		// We've got an error. It may or may not be our timeout error.
		// Either which way, we need to cancel the context.
		defer cancelFn()

		if errors.Is(context.Cause(ctx), timeoutErr) {
			// If it is our timeout error, we log it.

			lg.FromContext(ctx).Warn("HTTP request not completed within timeout",
				lga.Timeout, timeout, lga.URL, req.URL.String())
		}

		return resp, err
	}
}

// OptRequestTimeout is passed to NewClient to set a timeout for just
// getting the initial response headers. This is useful if you expect
// a response within, say, 2 seconds, but you expect the body to take longer
// to read.
//
// Contrast with OptResponseTimeout.
func OptRequestTimeout(timeout time.Duration) TripFunc {
	if timeout <= 0 {
		return NopTripFunc
	}
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		ctx, cancelFn := context.WithCancelCause(req.Context())

		// armed gates the header-timeout cancellation. The timer cancels the
		// context only if it fires while still armed, i.e. before RoundTrip has
		// returned. Once RoundTrip returns (success or error) we disarm, after
		// which the timer can never cancel the context. This matters because the
		// returned response body reads through this same context and may
		// legitimately take longer than the header timeout: a healthy body read
		// must not be aborted just because the header timer fires in the narrow
		// window around RoundTrip returning.
		var armed atomic.Bool
		armed.Store(true)

		t := time.AfterFunc(timeout, func() {
			// CompareAndSwap is the single source of truth for "did the timeout
			// win the race". It succeeds only if the timer fired while still armed,
			// i.e. before RoundTrip returned and disarmed it: a genuine header
			// timeout, so warn and cancel. If it fails, RoundTrip already returned;
			// the timer must not cancel the context the body reads through.
			if armed.CompareAndSwap(true, false) {
				lg.FromContext(ctx).Warn("HTTP header response not received within timeout",
					lga.Timeout, timeout, lga.URL, req.URL.String())
				cancelFn(errz.Wrapf(context.DeadlineExceeded,
					"http response header not received within %s timeout", timeout))
			}
		})

		// If next.RoundTrip panics, the normal cleanup below is skipped, which
		// would leave the context uncancelled. Disarm the timer and release the
		// context before the panic propagates. On the normal path recover() is
		// nil, and disarm/Stop are idempotent with the cleanup below.
		defer func() {
			if r := recover(); r != nil {
				armed.Store(false)
				t.Stop()
				cancelFn(context.Canceled)
				panic(r)
			}
		}()

		resp, err := errz.Return(next.RoundTrip(req.WithContext(ctx)))
		t.Stop()

		if !armed.CompareAndSwap(true, false) {
			// The timer won the race: it fired and cancelled the context before we
			// could disarm it, so this is a genuine header timeout. Even if
			// RoundTrip happened to return a response, the context is cancelled and
			// the body reads through it, so the response is unusable. Surface the
			// timeout consistently (never a response with a dead body) and release
			// the response. context.Cause holds the descriptive error set by the
			// timer.
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if cause := context.Cause(ctx); cause != nil {
				err = cause
			}
			return nil, err
		}

		// The timer is disarmed and can no longer cancel the context. Any
		// cancellation observed now comes from the parent context.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if langz.Take(ctx.Done()) {
				// The lower-down RoundTripper probably returned ctx.Err(),
				// not context.Cause(), so we swap it around here.
				if cause := context.Cause(ctx); cause != nil {
					err = cause
				}
			}
		}

		// Don't leak resources; ensure that cancelFn is eventually called. Use a
		// neutral cause: stamping DeadlineExceeded here would fabricate a
		// misleading cause on non-timeout errors.
		switch {
		case err != nil:
			cancelFn(context.Canceled)
		case resp != nil && resp.Body != nil:
			// Wrap resp.Body with a ReadCloserNotifier, so that cancelFn
			// is called when the body is closed.
			resp.Body = ioz.ReadCloserNotifier(resp.Body,
				func(error) { cancelFn(context.Canceled) })
		default:
			// No body to wait on (e.g. a HEAD response): release immediately.
			cancelFn(context.Canceled)
		}

		return resp, err
	}
}

// DefaultHeaderTimeout is the default header timeout as used
// by NewDefaultClient.
var DefaultHeaderTimeout = OptRequestTimeout(time.Second * 5)

// OptRequestDelay is passed to NewClient to delay the request by the
// specified duration. This is useful for testing.
func OptRequestDelay(delay time.Duration) TripFunc {
	if delay <= 0 {
		return NopTripFunc
	}

	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		ctx := req.Context()
		log := lg.FromContext(ctx)
		log.Debug("HTTP request delay: started", lga.Val, delay, lga.URL, req.URL.String())
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return nil, context.Cause(ctx)
		case <-t.C:
			// Continue below
		}

		log.Debug("HTTP request delay: done", lga.Val, delay, lga.URL, req.URL.String())
		return next.RoundTrip(req)
	}
}

// contextCause returns a TripFunc that extracts the context.Cause error
// from the request context, if any, and returns it as the error.
func contextCause() TripFunc {
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(req)
		if err != nil {
			if cause := context.Cause(req.Context()); cause != nil {
				err = cause
			}
		}
		return resp, err
	}
}
