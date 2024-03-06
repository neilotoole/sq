package httpz

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
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
		tr.TLSClientConfig.MinVersion = uint16(v)
	}
}

// DefaultTLSVersion is the default minimum TLS version,
// as used by NewDefaultClient.
var DefaultTLSVersion = minTLSVersion(tls.VersionTLS10)

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
			if resp.Body == nil {
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
		timerCancelCh := make(chan struct{})

		ctx, cancelFn := context.WithCancelCause(req.Context())
		t := time.NewTimer(timeout)
		go func() {
			defer t.Stop()
			select {
			case <-ctx.Done():
			case <-t.C:
				cancelErr := errz.Wrapf(context.DeadlineExceeded,
					"http response header not received within %s timeout", timeout)

				lg.FromContext(ctx).Warn("HTTP header response not received within timeout",
					lga.Timeout, timeout, lga.URL, req.URL.String())

				cancelFn(cancelErr)
			case <-timerCancelCh:
				// Stop the timer goroutine.
			}
		}()

		resp, err := errz.Return(next.RoundTrip(req.WithContext(ctx)))

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if langz.Take(ctx.Done()) {
				// The lower-down RoundTripper probably returned ctx.Err(),
				// not context.Cause(), so we swap it around here.
				if cause := context.Cause(ctx); cause != nil {
					err = cause
				}
			}
		}

		// Signal completion of the timer goroutine (it may have already completed).
		close(timerCancelCh)

		// Don't leak resources; ensure that cancelFn is eventually called.
		switch {
		case err != nil:
			// An error has occurred. It's probable that cancelFn has already been
			// called by the timer goroutine, but we call it again just in case.
			cancelFn(context.DeadlineExceeded)
		case resp != nil && resp.Body != nil:
			// Wrap resp.Body with a ReadCloserNotifier, so that cancelFn
			// is called when the body is closed.
			resp.Body = ioz.ReadCloserNotifier(resp.Body,
				func(error) { cancelFn(context.DeadlineExceeded) })
		default:
			// Not sure if this can actually be reached, but just in case.
			cancelFn(context.DeadlineExceeded)
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
