package rqlite

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
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
//     TLS-only servers that hang up instead of answering.
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
