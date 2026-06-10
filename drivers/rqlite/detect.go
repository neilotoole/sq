package rqlite

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// probeIndicatesTLS reports whether the outcome of a plain-HTTP
// probe suggests the endpoint actually speaks TLS. Unlike
// isTLSSignal (which classifies gorqlite-serialized error strings,
// where the error chain is broken), this classifier sees raw
// net/http results, so the typed checks fire.
//
// Signals, in order of likelihood:
//
//  1. HTTP 400 whose body is Go's canonical "Client sent an HTTP
//     request to an HTTPS server" response: a net/http TLS listener
//     answers plaintext HTTP this way, as a normal response, not an
//     error.
//  2. A redirect (301/302/307/308) to an https:// URL on the same
//     host: TLS-terminating proxies do this. The probe client must
//     not follow redirects, or this signal is invisible and the
//     endpoint is mis-detected as plain HTTP.
//  3. Transport-level tls.RecordHeaderError or EOF mid-response:
//     TLS-only servers that hang up instead of answering. EOF is a
//     deliberately weak signal: any abrupt close matches, not only
//     TLS hang-ups, so a flaky plain-HTTP server can trigger an
//     unnecessary HTTPS probe. The strict statusLooksRqlite
//     fingerprint on the HTTPS response is the second line of
//     defense against a false tls=true.
func probeIndicatesTLS(resp *http.Response, body []byte, err error) bool {
	if err != nil {
		var rec tls.RecordHeaderError
		if errors.As(err, &rec) {
			return true
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return true
		}
		return false
	}
	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		locHdr, lerr := resp.Location()
		if lerr == nil && locHdr.Scheme == "https" &&
			resp.Request != nil && resp.Request.URL != nil &&
			locHdr.Hostname() == resp.Request.URL.Hostname() {
			return true
		}
	case http.StatusBadRequest:
		if strings.Contains(string(body), "HTTP request to an HTTPS server") {
			return true
		}
	}
	return false
}

// statusLooksRqlite reports whether resp/body look like a genuine
// rqlite /status payload: HTTP 200, JSON content type, and a JSON
// object with a top-level "node" key. This prevents an unrelated
// HTTP server on the target port from being "confirmed" as rqlite.
func statusLooksRqlite(resp *http.Response, body []byte) bool {
	if resp == nil || resp.StatusCode != http.StatusOK {
		return false
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return false
	}
	_, ok := m["node"]
	return ok
}

// Compile-time check: driveri implements driver.ConnParamDetector.
var _ driver.ConnParamDetector = (*driveri)(nil)

// DetectConnParams implements driver.ConnParamDetector. It probes
// GET /status over plain HTTP first; if the failure indicates a
// TLS-only endpoint, it re-probes over HTTPS with standard cert
// verification. A confirmed HTTPS rqlite endpoint yields
// {"tls": ["true"]}. An HTTPS endpoint with an unverifiable cert
// yields an actionable error. Everything else steps aside with
// (nil, nil): the Ping that follows during "sq add" reports
// connection problems with full enrichment.
func (d *driveri) DetectConnParams(ctx context.Context, src *source.Source) (url.Values, error) {
	return d.detectConnParams(ctx, src, nil)
}

// detectConnParams is the testable core of DetectConnParams. If
// transport is non-nil it is used for both probe attempts; tests
// pass an httptest transport that trusts the test server's cert.
// A nil transport means http.DefaultTransport (with default cert
// verification).
func (d *driveri) detectConnParams(ctx context.Context, src *source.Source,
	transport http.RoundTripper,
) (url.Values, error) {
	log := lg.FromContext(ctx)

	loc, _, err := locationWithDefaultPort(src.Location)
	if err != nil {
		// Malformed location: not detection's concern.
		return nil, nil //nolint:nilerr,nilnil
	}
	u, err := url.Parse(loc)
	if err != nil {
		return nil, nil //nolint:nilerr,nilnil
	}
	if q := u.Query(); q.Has("tls") || q.Has("insecure") {
		log.Debug("rqlite: conn param detection skipped: explicit tls/insecure intent",
			lga.Src, src.Handle)
		return nil, nil //nolint:nilnil
	}

	client := &http.Client{
		// clientTimeout honors the gorqlite-native ?timeout=N URL
		// param (integer seconds) over conn.open-timeout, keeping the
		// probe consistent with the documented timeout behavior.
		Timeout:   clientTimeout(loc, src.Options),
		Transport: transport, // nil means http.DefaultTransport
		// Do not follow redirects: a redirect to https:// is itself
		// a TLS signal that probeIndicatesTLS must see; following it
		// silently would mis-detect an HTTPS-only endpoint as
		// HTTP-fine.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: plain HTTP.
	resp, body, err := probeStatus(ctx, client, u, "http") //nolint:bodyclose
	if err == nil && statusLooksRqlite(resp, body) {
		// Endpoint speaks HTTP; the default is already correct.
		log.Debug("rqlite: conn param detection: endpoint speaks plain HTTP",
			lga.Src, src.Handle)
		return nil, nil //nolint:nilnil
	}
	if !probeIndicatesTLS(resp, body, err) {
		if ctx.Err() != nil {
			return nil, errz.Err(ctx.Err())
		}
		log.Debug("rqlite: conn param detection: no TLS signal; stepping aside",
			lga.Src, src.Handle)
		return nil, nil //nolint:nilnil
	}

	// Step 2: HTTPS with standard cert verification.
	resp2, body2, err2 := probeStatus(ctx, client, u, "https") //nolint:bodyclose
	switch {
	case err2 == nil && statusLooksRqlite(resp2, body2):
		log.Debug("rqlite: conn param detection: TLS endpoint confirmed",
			lga.Src, src.Handle)
		return url.Values{"tls": []string{"true"}}, nil
	case isCertVerificationError(err2):
		// The one case detection reports better than Ping: we KNOW
		// the endpoint wants TLS and we KNOW the cert won't verify,
		// so the user gets the full prescription in one shot. The
		// wrapped err2 is a url.Error whose message embeds the probe
		// URL, which is credential-free (probeStatus strips
		// userinfo into the Authorization header).
		hint := suggestLocWithParams(src, url.Values{
			"tls": {"true"}, "insecure": {"true"},
		})
		return nil, errz.Wrapf(err2,
			"%s: the endpoint requires TLS, but its certificate could not "+
				"be verified. If this is a self-signed or private-CA "+
				"deployment, retry with %s, or install the CA in your "+
				"trust store", src.Handle, hint)
	default:
		if ctx.Err() != nil {
			return nil, errz.Err(ctx.Err())
		}
		log.Debug("rqlite: conn param detection: HTTPS probe inconclusive; stepping aside",
			lga.Src, src.Handle)
		return nil, nil //nolint:nilnil
	}
}

// probeStatus issues GET <scheme>://host/status with basic auth
// from u's userinfo, returning the response and its (bounded) body.
// Credentials travel in the Authorization header, never in the
// request URL, so error messages that embed the URL are
// credential-free.
func probeStatus(ctx context.Context, client *http.Client, u *url.URL, scheme string,
) (*http.Response, []byte, error) {
	pu := *u
	pu.Scheme = scheme
	pu.Path = "/status"
	pu.RawQuery = ""
	pu.Fragment = ""
	user := pu.User
	pu.User = nil

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pu.String(), nil)
	if err != nil {
		return nil, nil, errz.Err(err)
	}
	if user != nil {
		// Send auth whenever userinfo is present, including the
		// password-only form (rqlite://:pw@host): gorqlite sends the
		// same header at connection time, and the probe must match
		// so auth-gated /status endpoints behave consistently.
		pw, _ := user.Password()
		req.SetBasicAuth(user.Username(), pw)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err // raw chain: the classifiers need errors.As to work
	}
	defer func() { _ = resp.Body.Close() }()

	const maxBody = 64 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp, nil, err // raw chain: the classifiers need errors.As to work
	}
	return resp, body, nil
}
