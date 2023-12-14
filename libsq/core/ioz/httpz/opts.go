package httpz

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
)

// Opt is an option that can be passed to [NewClient] to
// configure the client.
type Opt interface {
	apply(*http.Transport)
}

var _ Opt = (*OptInsecureSkipVerify)(nil)

// OptInsecureSkipVerify is an Opt that can be passed to NewClient that,
// when true, disables TLS verification.
type OptInsecureSkipVerify bool

func (b OptInsecureSkipVerify) apply(tr *http.Transport) {
	tr.TLSClientConfig.InsecureSkipVerify = bool(b)
}

var _ Opt = (*minTLSVersion)(nil)

type minTLSVersion uint16

func (v minTLSVersion) apply(tr *http.Transport) {
	if tr.TLSClientConfig == nil {
		// We allow tls.VersionTLS10, even though it's not considered
		// secure these days. Ultimately this could become a config
		// option.
		tr.TLSClientConfig = &tls.Config{MinVersion: uint16(v)} //nolint:gosec
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
		tr.TLSClientConfig.MinVersion = uint16(v) //nolint:gosec
	}
}

// DefaultTLSVersion is the default minimum TLS version used by [NewClient].
var DefaultTLSVersion = minTLSVersion(tls.VersionTLS10)

// OptUserAgent is passed to [NewClient] to set the User-Agent header.
func OptUserAgent(ua string) TripFunc {
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", ua)
		return next.RoundTrip(req)
	}
}

// OptRequestTimeout is passed to [NewClient] to set the total request timeout.
// If timeout is zero, this is a no-op.
//
// Contrast with [OptHeaderTimeout].
func OptRequestTimeout(timeout time.Duration) TripFunc {
	if timeout <= 0 {
		return NopTripFunc
	}
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		ctx, cancelFn := context.WithTimeoutCause(req.Context(), timeout,
			errz.Wrapf(context.DeadlineExceeded, "http request not completed within %s timeout", timeout))
		defer cancelFn()
		resp, err := next.RoundTrip(req.WithContext(ctx))
		if err == nil {
			return resp, nil
		}

		if errors.Is(err, ctx.Err()) {
			// The lower-down RoundTripper probably returned ctx.Err(),
			// not context.Cause(), so we swap it around here.
			err = context.Cause(ctx)
		}
		return resp, err
	}
}

// OptHeaderTimeout is passed to [NewClient] to set a timeout for just
// getting the initial response headers. This is useful if you expect
// a response within, say, 5 seconds, but you expect the body to take longer
// to read. If bodyTimeout > 0, it is applied to the total lifecycle of
// the request and response, including reading the response body.
// If timeout <= zero, this is a no-op.
//
// Contrast with [OptRequestTimeout].
func OptHeaderTimeout(timeout time.Duration) TripFunc {
	if timeout <= 0 {
		return NopTripFunc
	}
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		timerCancelCh := make(chan struct{})
		ctx, cancelFn := context.WithCancelCause(req.Context())
		go func() {
			t := time.NewTimer(timeout)
			defer t.Stop()
			select {
			case <-ctx.Done():
			case <-t.C:
				cancelErr := errz.Wrapf(context.DeadlineExceeded,
					"http response not received within %s timeout",
					timeout)
				cancelFn(cancelErr)
			case <-timerCancelCh:
				// Stop the timer goroutine.
			}
		}()

		resp, err := next.RoundTrip(req.WithContext(ctx))
		close(timerCancelCh)
		if err != nil && errors.Is(err, ctx.Err()) {
			// The lower-down RoundTripper probably returned ctx.Err(),
			// not context.Cause(), so we swap it around here.
			err = context.Cause(ctx)
		}
		// Don't leak resources; ensure that cancelFn is eventually called.
		switch {
		case err != nil:
			// It's probable that cancelFn has already been called by the
			// timer goroutine, but we call it again just in case.
			cancelFn(err)
		case resp != nil && resp.Body != nil:

			// Wrap resp.Body with a ReadCloserNotifier, so that cancelFn
			// is called when the body is closed.
			resp.Body = ioz.ReadCloserNotifier(resp.Body, cancelFn)
		default:
			// Not sure if this can actually happen, but just in case.
			cancelFn(context.Canceled)
		}

		return resp, err
	}
}
