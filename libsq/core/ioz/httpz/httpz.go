// Package httpz provides functionality supplemental to stdlib http.
// Indeed, some of the functions are copied verbatim from stdlib.
// The jumping-off point is [httpz.NewClient].
//
// Design note: this package contains generally fairly straightforward HTTP
// functionality, but the Opt / TripFunc  config mechanism is a bit
// experimental. And probably tries to be a bit too clever. It may change.
//
// And one last thing: remember kids, ALWAYS close your response bodies.
package httpz

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/textproto"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

// NewDefaultClient invokes NewClient with default settings.
func NewDefaultClient() *http.Client {
	return NewClient(
		OptInsecureSkipVerify(false),
		DefaultUserAgent,
		DefaultHeaderTimeout,
	)
}

// NewClient returns a new HTTP client configured with opts.
func NewClient(opts ...Opt) *http.Client {
	c := *http.DefaultClient
	c.Timeout = 0
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
	// Apply the round trip functions in reverse order.
	for i := len(opts) - 1; i >= 0; i-- {
		if tf, ok := opts[i].(TripFunc); ok {
			c.Transport = RoundTrip(c.Transport, tf)
		}
	}
	c.Transport = RoundTrip(c.Transport, contextCause())
	return &c
}

var _ Opt = (*TripFunc)(nil)

// TripFunc is a function that implements http.RoundTripper.
// It is commonly used with RoundTrip to decorate an existing http.RoundTripper.
type TripFunc func(next http.RoundTripper, req *http.Request) (*http.Response, error)

func (tf TripFunc) apply(*http.Transport) {}

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

// ResponseLogValue implements slog.LogValuer for resp.
func ResponseLogValue(resp *http.Response) slog.Value {
	if resp == nil {
		return slog.Value{}
	}

	attrs := []slog.Attr{
		slog.String("proto", resp.Proto),
		slog.String("status", resp.Status),
	}

	h := resp.Header
	for k := range h {
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

// Log logs req, resp, and err via the logger on req.Context().
func Log(req *http.Request, resp *http.Response, err error) {
	log := lg.FromContext(req.Context()).With(lga.Method, req.Method, lga.URL, req.URL)
	if err != nil {
		log.Warn("HTTP request error", lga.Err, err)
		return
	}

	log.Debug("HTTP request completed", lga.Resp, ResponseLogValue(resp))
}

// RequestLogValue implements slog.LogValuer for req.
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
	for k := range h {
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
		if err == io.EOF { //nolint:errorlint
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
		if err == io.EOF { //nolint:errorlint
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

// StatusText is like http.StatusText, but also includes the code, e.g. "200/OK".
func StatusText(code int) string {
	return strconv.Itoa(code) + "/" + http.StatusText(code)
}
