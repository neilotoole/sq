package rqlite

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func mkResp(status int, contentType, reqURL string) *http.Response {
	u, _ := url.Parse(reqURL)
	resp := &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Request:    &http.Request{URL: u},
	}
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}
	return resp
}

func TestProbeIndicatesTLS(t *testing.T) {
	const canonical400 = "Client sent an HTTP request to an HTTPS server.\n"

	t.Run("nil resp nil err", func(t *testing.T) {
		require.False(t, probeIndicatesTLS(nil, nil, nil))
	})

	t.Run("tls record header error", func(t *testing.T) {
		require.True(t, probeIndicatesTLS(nil, nil, tls.RecordHeaderError{Msg: "x"}))
	})

	t.Run("io.EOF", func(t *testing.T) {
		require.True(t, probeIndicatesTLS(nil, nil, io.EOF))
	})

	t.Run("io.ErrUnexpectedEOF", func(t *testing.T) {
		require.True(t, probeIndicatesTLS(nil, nil, io.ErrUnexpectedEOF))
	})

	t.Run("unrelated error", func(t *testing.T) {
		require.False(t, probeIndicatesTLS(nil, nil, errz.New("conn refused")))
	})

	t.Run("400 with canonical body", func(t *testing.T) {
		resp := mkResp(400, "text/plain", "http://h:4001/status")
		require.True(t, probeIndicatesTLS(resp, []byte(canonical400), nil))
	})

	t.Run("400 with other body", func(t *testing.T) {
		resp := mkResp(400, "text/plain", "http://h:4001/status")
		require.False(t, probeIndicatesTLS(resp, []byte("bad request"), nil))
	})

	t.Run("redirect to https same host", func(t *testing.T) {
		resp := mkResp(301, "", "http://h:4001/status")
		resp.Header.Set("Location", "https://h:4001/status")
		require.True(t, probeIndicatesTLS(resp, nil, nil))
	})

	t.Run("redirect to https different host", func(t *testing.T) {
		resp := mkResp(301, "", "http://h:4001/status")
		resp.Header.Set("Location", "https://other.example.com/status")
		require.False(t, probeIndicatesTLS(resp, nil, nil))
	})

	t.Run("redirect to http is not a tls signal", func(t *testing.T) {
		resp := mkResp(302, "", "http://h:4001/status")
		resp.Header.Set("Location", "http://h:4001/other")
		require.False(t, probeIndicatesTLS(resp, nil, nil))
	})

	t.Run("plain 200 is not a tls signal", func(t *testing.T) {
		resp := mkResp(200, "application/json", "http://h:4001/status")
		require.False(t, probeIndicatesTLS(resp, []byte("{}"), nil))
	})
}

func TestStatusLooksRqlite(t *testing.T) {
	t.Run("rqlite-shaped json", func(t *testing.T) {
		resp := mkResp(200, "application/json", "http://h/status")
		require.True(t, statusLooksRqlite(resp, []byte(`{"node":{"start_time":"x"}}`)))
	})

	t.Run("non-200", func(t *testing.T) {
		resp := mkResp(401, "application/json", "http://h/status")
		require.False(t, statusLooksRqlite(resp, []byte(`{"node":{}}`)))
	})

	t.Run("wrong content type", func(t *testing.T) {
		resp := mkResp(200, "text/html", "http://h/status")
		require.False(t, statusLooksRqlite(resp, []byte(`{"node":{}}`)))
	})

	t.Run("json without node key", func(t *testing.T) {
		resp := mkResp(200, "application/json", "http://h/status")
		require.False(t, statusLooksRqlite(resp, []byte(`{"version":"8"}`)))
	})

	t.Run("non-json body", func(t *testing.T) {
		resp := mkResp(200, "application/json", "http://h/status")
		require.False(t, statusLooksRqlite(resp, []byte(`<html></html>`)))
	})

	t.Run("nil resp", func(t *testing.T) {
		require.False(t, statusLooksRqlite(nil, []byte(`{"node":{}}`)))
	})
}

// countingHandler wraps h, counting requests.
func countingHandler(h http.Handler, n *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		h.ServeHTTP(w, r)
	})
}

func detectTestSrc(loc string) *source.Source {
	return &source.Source{
		Handle:   "@rq",
		Type:     drivertype.Rqlite,
		Location: loc,
	}
}

func TestDetectConnParams_PlainHTTP(t *testing.T) {
	var host string
	var hits atomic.Int64
	server := httptest.NewServer(countingHandler(newRqliteMockHandler(&host), &hits))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.detectConnParams(ctx, detectTestSrc("rqlite://"+host), nil)
	require.NoError(t, err)
	require.Nil(t, params, "plain-HTTP endpoint: nothing to detect")
	require.Equal(t, int64(1), hits.Load(), "exactly one probe request")
}

func TestDetectConnParams_TLS(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	// Inject the test server's transport, which trusts its cert.
	transport := server.Client().Transport
	params, err := d.detectConnParams(ctx, detectTestSrc("rqlite://"+host), transport)
	require.NoError(t, err)
	require.Equal(t, url.Values{"tls": []string{"true"}}, params)
}

func TestDetectConnParams_TLSBadCert(t *testing.T) {
	var host string
	server := httptest.NewTLSServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	// nil transport: default verification distrusts the test cert.
	_, err := d.detectConnParams(ctx, detectTestSrc("rqlite://"+host), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure=true")
	require.Contains(t, err.Error(), "tls=true")
}

func TestDetectConnParams_ExplicitIntentSkips(t *testing.T) {
	var host string
	var hits atomic.Int64
	server := httptest.NewServer(countingHandler(newRqliteMockHandler(&host), &hits))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	for _, loc := range []string{
		"rqlite://" + host + "?tls=true",
		"rqlite://" + host + "?tls=false",
		"rqlite://" + host + "?tls=true&insecure=true",
	} {
		params, err := d.detectConnParams(ctx, detectTestSrc(loc), nil)
		require.NoError(t, err)
		require.Nil(t, params)
	}
	require.Equal(t, int64(0), hits.Load(), "explicit intent must mean zero probes")
}

func TestDetectConnParams_NonRqliteServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>hello</html>"))
		}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://"+server.Listener.Addr().String()), nil)
	require.NoError(t, err)
	require.Nil(t, params, "non-rqlite server: step aside")
}

// TestDetectConnParams_NonRqliteJSON covers the JSON-but-not-rqlite
// case end-to-end: a 200 application/json response without a
// top-level "node" key must not be confirmed as rqlite, must not
// trigger the HTTPS fallback, and must step aside.
func TestDetectConnParams_NonRqliteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"8","status":"ok"}`))
		}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://"+server.Listener.Addr().String()), nil)
	require.NoError(t, err)
	require.Nil(t, params, "non-rqlite JSON server: step aside")
}

func TestDetectConnParams_Unreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	// Port 1 on localhost: reliably refused.
	params, err := d.detectConnParams(ctx, detectTestSrc("rqlite://127.0.0.1:1"), nil)
	require.NoError(t, err)
	require.Nil(t, params, "unreachable endpoint: step aside, Ping reports it")
}

func TestDetectConnParams_RedirectToHTTPS(t *testing.T) {
	// httptest cannot serve http and https on one port, so this test
	// verifies the redirect CLASSIFICATION path end-to-end: the front
	// is a plain-HTTP server that redirects to https:// on its own
	// host:port. Step 1 sees the same-host https redirect (a TLS
	// signal; the probe client must NOT follow it), step 2 probes
	// https:// against the plain listener, fails the handshake, and
	// steps aside. The front's hit count proves the redirect was not
	// followed: exactly one request.
	var hits atomic.Int64
	front := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			target := "https://" + r.Host + r.URL.Path
			http.Redirect(w, r, target, http.StatusMovedPermanently)
		}))
	t.Cleanup(front.Close)
	host := front.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.detectConnParams(ctx, detectTestSrc("rqlite://"+host), nil)
	require.NoError(t, err)
	require.Nil(t, params)
	require.Equal(t, int64(1), hits.Load(),
		"probe must not follow the redirect: exactly one request to the front")
}

func TestDetectConnParams_CtxCanceled(t *testing.T) {
	var host string
	server := httptest.NewServer(newRqliteMockHandler(&host))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the probe starts

	d := &driveri{}
	_, err := d.detectConnParams(ctx, detectTestSrc("rqlite://"+host), nil)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestDetectConnParams_BasicAuth(t *testing.T) {
	var host string
	var gotAuth atomic.Bool
	inner := newRqliteMockHandler(&host)
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if u, p, ok := r.BasicAuth(); ok && u == "alice" && p == "pw" {
				gotAuth.Store(true)
			}
			inner.ServeHTTP(w, r)
		}))
	t.Cleanup(server.Close)
	host = server.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	_, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://alice:pw@"+host), nil)
	require.NoError(t, err)
	require.True(t, gotAuth.Load(), "probe must carry basic auth from the location userinfo")
}
