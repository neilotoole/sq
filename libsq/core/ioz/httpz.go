package ioz

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewHTTPClient returns a new HTTP client. If userAgent is non-empty, the
// "User-Agent" header is applied to each request. If insecureSkipVerify is
// true, the client will skip TLS verification. If headerTimeout > 0, a
// timeout is applied to receiving the HTTP response, but that timeout is
// not applied to reading the response body. This is useful if you expect
// a response within, say, 5 seconds, but you expect the body to take longer
// to read. If bodyTimeout > 0, it is applied to the total lifecycle of
// the request and response, including reading the response body.
func NewHTTPClient(userAgent string, insecureSkipVerify bool,
	headerTimeout, bodyTimeout time.Duration,
) *http.Client {
	c := *http.DefaultClient
	var tr *http.Transport
	if c.Transport == nil {
		tr = (http.DefaultTransport.(*http.Transport)).Clone()
	} else {
		tr = (c.Transport.(*http.Transport)).Clone()
	}

	if tr.TLSClientConfig == nil {
		// We allow tls.VersionTLS10, even though it's not considered
		// secure these days. Ultimately this could become a config
		// option.
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS10} //nolint:gosec
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
		tr.TLSClientConfig.MinVersion = tls.VersionTLS10 //nolint:gosec
	}

	tr.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify
	c.Transport = tr
	if userAgent != "" {
		c.Transport = &userAgentRoundTripper{
			userAgent: userAgent,
			rt:        c.Transport,
		}
	}

	c.Timeout = bodyTimeout
	if headerTimeout > 0 {
		c.Transport = &headerTimeoutRoundTripper{
			headerTimeout: headerTimeout,
			rt:            c.Transport,
		}
	}

	return &c
}

// userAgentRoundTripper applies a User-Agent header to each request.
type userAgentRoundTripper struct {
	userAgent string
	rt        http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (rt *userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", rt.userAgent)
	return rt.rt.RoundTrip(req)
}

// headerTimeoutRoundTripper applies headerTimeout to the return of the http
// response, but headerTimeout is not applied to reading the body of the
// response. This is useful if you expect a response within, say, 5 seconds,
// but you expect the body to take longer to read.
type headerTimeoutRoundTripper struct {
	headerTimeout time.Duration
	rt            http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (rt *headerTimeoutRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.headerTimeout <= 0 {
		return rt.rt.RoundTrip(req)
	}

	timerCancelCh := make(chan struct{})
	ctx, cancelFn := context.WithCancelCause(req.Context())
	go func() {
		t := time.NewTimer(rt.headerTimeout)
		defer t.Stop()
		select {
		case <-ctx.Done():
		case <-t.C:
			cancelFn(errz.Errorf("http response not received by %d timeout",
				rt.headerTimeout))
		case <-timerCancelCh:
			// Stop the timer goroutine.
		}
	}()

	resp, err := rt.rt.RoundTrip(req.WithContext(ctx))
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
		resp.Body = ReadCloserNotifier(resp.Body, cancelFn)
	default:
		// Not sure if this can actually happen, but just in case.
		cancelFn(context.Canceled)
	}

	return resp, err
}
