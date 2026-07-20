package rqlite

import (
	"context"
	"crypto/tls"
	"database/sql/driver"
	"net/http"
	"time"

	"github.com/rqlite/gorqlite"
	"github.com/rqlite/gorqlite/stdlib"
)

// insecureConnector is a database/sql connector that opens gorqlite
// connections with a custom http.Client. It exists to support
// ?insecure=true (TLS cert verification skipped), which the
// stdlib package's default sql.Open("rqlite", dsn) path cannot
// express because that path constructs gorqlite's default client.
type insecureConnector struct {
	client *http.Client
	dsn    string
}

// Connect implements driver.Connector.
func (c *insecureConnector) Connect(_ context.Context) (driver.Conn, error) {
	conn, err := gorqlite.OpenWithClient(c.dsn, c.client)
	if err != nil {
		// Strip any *url.Error wrapper first: c.dsn carries inline
		// userinfo, and gorqlite returns a bare net/url parse error for a
		// bad URL, whose message embeds the raw URL (password included).
		// Mirrors dsnFromLocation's redaction guarantee.
		return nil, errw(stripURLError(err))
	}
	// Wrap with sqConn (matching sqDriver.Open) so the
	// ColumnTypeDatabaseTypeName enrichment from sqRows is in
	// effect on the insecure path too. Without this wrap, every
	// column kind resolves to kind.Unknown.
	return &sqConn{Conn: &stdlib.Conn{Connection: conn}}, nil
}

// Driver implements driver.Connector. Returns sqDriver (the same
// driver registered as sqDBDrvrName) so sql.DB.Driver() introspection
// is consistent between the default and insecure code paths.
func (c *insecureConnector) Driver() driver.Driver {
	return &sqDriver{inner: &stdlib.Driver{}}
}

// newInsecureHTTPClient returns an *http.Client whose TLS config
// has InsecureSkipVerify set, with the given timeout. Intended only
// for the ?insecure=true code path.
//
// http.DefaultTransport is cloned so that ProxyFromEnvironment, HTTP/2
// support, keepalive tuning, and dial timeouts are preserved. Only the
// TLSClientConfig is overridden.
func newInsecureHTTPClient(timeout time.Duration) *http.Client {
	// http.DefaultTransport is documented as *http.Transport, but
	// test harnesses sometimes replace it. Fall back to a fresh
	// *http.Transport on the (unreachable in production) path.
	var transport *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = dt.Clone()
	} else {
		transport = &http.Transport{}
	}
	transport.TLSClientConfig = &tls.Config{
		//nolint:gosec // ?insecure=true is an explicit user opt-in
		InsecureSkipVerify: true,
		// MinVersion is set explicitly as defense in depth; TLS 1.2 is the
		// modern floor even when certificate verification is skipped.
		MinVersion: tls.VersionTLS12,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
