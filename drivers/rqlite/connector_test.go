package rqlite

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// newRqliteMockHandler returns a handler that satisfies gorqlite's cluster
// discovery protocol. hostPtr is a pointer to the "host:port" string of the
// test server; it is resolved at request time so the handler can be
// constructed before the server has a URL. The handler responds to:
//   - GET /status  — returns a minimal JSON body with an empty store.leader so
//     gorqlite falls through to the /nodes fallback.
//   - GET /nodes   — returns the test server itself as the single reachable
//     leader, using *hostPtr resolved at call time.
//   - GET /db/query — returns a no-op single-row result (used by PingContext).
func newRqliteMockHandler(hostPtr *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/status":
			// Return a store.leader raft addr with no matching metadata api_addr
			// so gorqlite successfully parses the leader but then falls back to
			// /nodes to resolve the HTTP API address.
			_, _ = w.Write([]byte(`{"node":{},"store":{"leader":"127.0.0.1:4002","metadata":{}}}`))
		case "/nodes":
			// Return the test server itself as the reachable leader.
			body := fmt.Sprintf(
				`{"1":{"api_addr":"https://%s","addr":"127.0.0.1:4002","reachable":true,"leader":true}}`,
				*hostPtr,
			)
			_, _ = w.Write([]byte(body))
		case "/db/query":
			_, _ = w.Write([]byte(`{"results":[{"columns":["1"],"types":["integer"],"values":[[1]]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestInsecureConnector_DriverReturnsSqDriver(t *testing.T) {
	c := &insecureConnector{dsn: "https://example.invalid", client: &http.Client{}}
	drvr := c.Driver()
	require.NotNil(t, drvr)
	_, ok := drvr.(*sqDriver)
	require.True(t, ok, "Driver() must return *sqDriver")
}

func TestInsecureConnector_SkipsVerifyOnSelfSignedCert(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	// Pass the full https:// URL; gorqlite's OpenWithClient accepts it directly.
	dsn := server.URL

	client := newInsecureHTTPClient(5 * time.Second)
	db := sql.OpenDB(&insecureConnector{dsn: dsn, client: client})
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	// gorqlite calls /status then /nodes during cluster discovery, so a
	// successful ping confirms the TLS request went through with InsecureSkipVerify.
	require.NoError(t, db.PingContext(ctx))
}

func TestInsecureConnector_VerifyingClientFailsOnSelfSignedCert(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	// A client WITHOUT InsecureSkipVerify must reject the test
	// server's self-signed cert. We check the error message for
	// "x509:" rather than type-asserting an exact x509 type because
	// the actual surfaced type depends on gorqlite's wrapping.
	verifyingClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{}, // default verification
		},
	}
	db := sql.OpenDB(&insecureConnector{dsn: server.URL, client: verifyingClient})
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	err := db.PingContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "x509:",
		"expected an x509 verification error, got: %v", err)
}

// TestDoOpen_InsecureSelectsInsecureConnector pins doOpen's dispatch:
// with ?tls=true&insecure=true the insecure connector (skip-verify)
// must be selected, so PingContext succeeds against a self-signed
// TLS server. Without insecure=true, the default sql.Open path runs
// cert verification and must fail.
func TestDoOpen_InsecureSelectsInsecureConnector(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	t.Run("insecure=true succeeds via skip-verify", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@rq_insecure",
			Type:     drivertype.Rqlite,
			Location: "rqlite://" + host + "?tls=true&insecure=true",
		}
		db, err := d.doOpen(ctx, src)
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		require.NoError(t, db.PingContext(ctx))
	})

	t.Run("without insecure the default path fails cert verification", func(t *testing.T) {
		src := &source.Source{
			Handle:   "@rq_verify",
			Type:     drivertype.Rqlite,
			Location: "rqlite://" + host + "?tls=true",
		}
		db, err := d.doOpen(ctx, src)
		require.NoError(t, err) // sql.Open is lazy.
		t.Cleanup(func() { _ = db.Close() })
		err = db.PingContext(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "x509:")
	})
}

func TestInsecureConnector_ColumnTypeDatabaseTypeName(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	client := newInsecureHTTPClient(5 * time.Second)
	db := sql.OpenDB(&insecureConnector{dsn: server.URL, client: client})
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	rows, err := db.QueryContext(ctx, "SELECT 1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })

	cts, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, cts, 1, "expected one column from SELECT 1")
	require.Equal(t, "integer", cts[0].DatabaseTypeName(),
		"DatabaseTypeName must be propagated through sqConn->sqStmt->sqRows; "+
			"an empty string means the insecure connector bypassed sqDriver")
}
