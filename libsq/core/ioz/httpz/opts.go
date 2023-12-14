package httpz

import (
	"context"
	"crypto/tls"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"net/http"
	"time"
)

// Opt is an option that can be passed to [NewClient2] to
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

// DefaultTLSVersion is the default minimum TLS version used by [NewClient2].
var DefaultTLSVersion = minTLSVersion(tls.VersionTLS10)

// OptUserAgent is passed to [NewClient2] to set the User-Agent header.
func OptUserAgent(ua string) TripFunc {
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		req.Header.Set("User-Agent", ua)
		return next.RoundTrip(req)
	}
}

// OptRequestTimeout is passed to [NewClient2] to set the total request timeout.
// If timeout is zero, this is a no-op.
//
// Contrast with [OptHeaderTimeout].
func OptRequestTimeout(timeout time.Duration) TripFunc {
	if timeout <= 0 {
		return NopTripFunc
	}
	return func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
		ctx, cancelFn := context.WithTimeoutCause(req.Context(), timeout,
			errz.Errorf("http request not completed in %s timeout", timeout))
		defer cancelFn()
		req = req.WithContext(ctx)
		return next.RoundTrip(req)
	}
}

// OptHeaderTimeout is passed to [NewClient2] to set a timeout for just
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
				cancelFn(errz.Errorf("http response not received by %s timeout",
					timeout))
			case <-timerCancelCh:
				// Stop the timer goroutine.
			}
		}()

		resp, err := next.RoundTrip(req.WithContext(ctx))
		close(timerCancelCh)

		// Don't leak resources; ensure that cancelFn is eventually called.
		switch {
		case err != nil:

			// It's possible that cancelFn has already been called by the
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
