// Package httpz provides functionality supplemental to stdlib http.
// Indeed, some of the functions are copied verbatim from stdlib.
package httpz

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/textproto"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewDefaultClient returns a new HTTP client with default settings.
func NewDefaultClient() *http.Client {
	return NewClient(
		buildinfo.Get().UserAgent(),
		true,
		0,
		0,
		OptUserAgent(buildinfo.Get().UserAgent()),
	)
} // NewDefaultClient returns a new HTTP client with default settings.
func NewDefaultClient2() *http.Client {
	return NewClient2(
		OptInsecureSkipVerify(true),
		OptUserAgent(buildinfo.Get().UserAgent()),
	)
}

// NewClient returns a new HTTP client. If userAgent is non-empty, the
// "User-Agent" header is applied to each request. If insecureSkipVerify is
// true, the client will skip TLS verification. If headerTimeout > 0, a
// timeout is applied to receiving the HTTP response, but that timeout is
// not applied to reading the response body. This is useful if you expect
// a response within, say, 5 seconds, but you expect the body to take longer
// to read. If bodyTimeout > 0, it is applied to the total lifecycle of
// the request and response, including reading the response body.
func NewClient(userAgent string, insecureSkipVerify bool,
	headerTimeout, bodyTimeout time.Duration, tripFuncs ...TripFunc,
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
	for i := range tripFuncs {
		c.Transport = RoundTrip(c.Transport, tripFuncs[i])
	}
	//
	//if userAgent != "" {
	//	//c.Transport = UserAgent2(c.Transport, userAgent)
	//
	//	//var funcs []TripFunc
	//
	//
	//
	//	//c.Transport = RoundTrip(c.Transport, func(next http.RoundTripper, req *http.Request) (*http.Response, error) {
	//	//	req.Header.Set("User-Agent", userAgent)
	//	//	return next.RoundTrip(req)
	//	//})
	//
	//	c.Transport = &userAgentRoundTripper{
	//		userAgent: userAgent,
	//		rt:        c.Transport,
	//	}
	//}
	//
	//c.Timeout = bodyTimeout
	//if headerTimeout > 0 {
	//	c.Transport = &headerTimeoutRoundTripper{
	//		headerTimeout: headerTimeout,
	//		rt:            c.Transport,
	//	}
	//}

	return &c
}

// NewClient2 returns a new HTTP client configured with opts.
func NewClient2(opts ...Opt) *http.Client {
	c := *http.DefaultClient
	var tr *http.Transport
	if c.Transport == nil {
		tr = (http.DefaultTransport.(*http.Transport)).Clone()
	} else {
		tr = (c.Transport.(*http.Transport)).Clone()
	}

	DefaultTLSVersion.apply(tr)
	for _, opt := range opts {
		opt.apply(tr)
	}

	c.Transport = tr
	for i := range opts {
		if tf, ok := opts[i].(TripFunc); ok {
			c.Transport = RoundTrip(c.Transport, tf)
		}
	}
	return &c
}

var _ Opt = (*TripFunc)(nil)

// TripFunc is a function that implements http.RoundTripper.
// It is commonly used with RoundTrip to decorate an existing http.RoundTripper.
type TripFunc func(next http.RoundTripper, req *http.Request) (*http.Response, error)

func (tf TripFunc) apply(tr *http.Transport) {}

// RoundTrip adapts a TripFunc to http.RoundTripper.
func RoundTrip(next http.RoundTripper, fn TripFunc) http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return fn(next, req)
	})
}

// NopTripFunc is a TripFunc that does nothing.
func NopTripFunc(next http.RoundTripper, req *http.Request) (*http.Response, error) {
	return next.RoundTrip(req)
}

// roundTripFunc is an adapter to allow use of functions as http.RoundTripper.
// It works with TripFunc and RoundTrip.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
			cancelFn(errz.Errorf("http response not received by %s timeout",
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
		resp.Body = ioz.ReadCloserNotifier(resp.Body, cancelFn)
	default:
		// Not sure if this can actually happen, but just in case.
		cancelFn(context.Canceled)
	}

	return resp, err
}

// ResponseLogValue implements slog.Valuer for resp.
func ResponseLogValue(resp *http.Response) slog.Value {
	if resp == nil {
		return slog.Value{}
	}

	attrs := []slog.Attr{
		slog.String("proto", resp.Proto),
		slog.String("status", resp.Status),
	}

	h := resp.Header
	for k, _ := range h {
		vals := h.Values(k)
		if len(vals) == 1 {
			attrs = append(attrs, slog.String(k, vals[0]))
			continue
		}

		attrs = append(attrs, slog.Any(k, h.Get(k)))
	}

	if resp.Request != nil {
		attrs = append(attrs, slog.Any("req", RequestLogValue(resp.Request)))
	}

	return slog.GroupValue(attrs...)
}

// RequestLogValue implements slog.Valuer for req.
func RequestLogValue(req *http.Request) slog.Value {
	if req == nil {
		return slog.Value{}
	}

	attrs := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("path", req.URL.RawPath),
	}

	if req.Proto != "" {
		attrs = append(attrs, slog.String("proto", req.Proto))
	}
	if req.Host != "" {
		attrs = append(attrs, slog.String("host", req.Host))
	}

	h := req.Header
	for k, _ := range h {
		vals := h.Values(k)
		if len(vals) == 1 {
			attrs = append(attrs, slog.String(k, vals[0]))
			continue
		}

		attrs = append(attrs, slog.Any(k, h.Get(k)))
	}

	return slog.GroupValue(attrs...)
}

// Filename returns the filename to use for a download.
// It first checks the Content-Disposition header, and if that's
// not present, it uses the last path segment of the URL. The
// filename is sanitized.
// It's possible that the returned value will be empty string; the
// caller should handle that situation themselves.
func Filename(resp *http.Response) string {
	var filename string
	if resp == nil || resp.Header == nil {
		return ""
	}
	dispHeader := resp.Header.Get("Content-Disposition")
	if dispHeader != "" {
		if _, params, err := mime.ParseMediaType(dispHeader); err == nil {
			filename = params["filename"]
		}
	}

	if filename == "" {
		filename = path.Base(resp.Request.URL.Path)
	} else {
		filename = filepath.Base(filename)
	}

	return stringz.SanitizeFilename(filename)
}

// ReadResponseHeader is a fork of http.ReadResponse that reads only the
// header from req and not the body. Note that resp.Body will be nil, and
// that the resp object is borked for general use.
func ReadResponseHeader(r *bufio.Reader, req *http.Request) (resp *http.Response, err error) {
	tp := textproto.NewReader(r)
	resp = &http.Response{Request: req}

	// Parse the first line of the response.
	line, err := tp.ReadLine()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	proto, status, ok := strings.Cut(line, " ")
	if !ok {
		return nil, badStringError("malformed HTTP response", line)
	}
	resp.Proto = proto
	resp.Status = strings.TrimLeft(status, " ")

	statusCode, _, _ := strings.Cut(resp.Status, " ")
	if len(statusCode) != 3 {
		return nil, badStringError("malformed HTTP status code", statusCode)
	}
	resp.StatusCode, err = strconv.Atoi(statusCode)
	if err != nil || resp.StatusCode < 0 {
		return nil, badStringError("malformed HTTP status code", statusCode)
	}
	if resp.ProtoMajor, resp.ProtoMinor, ok = http.ParseHTTPVersion(resp.Proto); !ok {
		return nil, badStringError("malformed HTTP version", resp.Proto)
	}

	// Parse the response headers.
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	resp.Header = http.Header(mimeHeader)

	fixPragmaCacheControl(resp.Header)

	return resp, nil
}

// RFC 7234, section 5.4: Should treat
//
//	Pragma: no-cache
//
// like
//
//	Cache-Control: no-cache
func fixPragmaCacheControl(header http.Header) {
	if hp, ok := header["Pragma"]; ok && len(hp) > 0 && hp[0] == "no-cache" {
		if _, presentcc := header["Cache-Control"]; !presentcc {
			header["Cache-Control"] = []string{"no-cache"}
		}
	}
}

func badStringError(what, val string) error { return fmt.Errorf("%s %q", what, val) }
