package rqlite

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
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
	// Keep in sync with Go's net/http TLS-listener response;
	// TestDetectConnParams_TLS exercises the real server-generated
	// form end-to-end.
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
	src := detectTestSrc("rqlite://" + host)
	ogLoc := src.Location
	params, err := d.detectConnParams(ctx, src, nil)
	require.NoError(t, err)
	require.Nil(t, params, "plain-HTTP endpoint: nothing to detect")
	require.Equal(t, int64(1), hits.Load(), "exactly one probe request")
	require.Equal(t, ogLoc, src.Location, "detector must not mutate src")
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
	src := detectTestSrc("rqlite://" + host)
	ogLoc := src.Location
	params, err := d.detectConnParams(ctx, src, transport)
	require.NoError(t, err)
	require.Equal(t, url.Values{"tls": []string{"true"}}, params)
	require.Equal(t, ogLoc, src.Location, "detector must not mutate src")
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
	_, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://alice:secret123@"+host), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure=true")
	require.Contains(t, err.Error(), "tls=true")
	require.NotContains(t, err.Error(), "secret123",
		"cert error must not echo the password from src.Location")

	// The human form carries the concise one-liner; the retry remedy
	// stays in the long form above.
	var hr errz.HumanReadable
	require.True(t, errors.As(err, &hr))
	require.Equal(t,
		"@rq: rqlite: endpoint requires TLS, but cert verification failed",
		hr.HumanError())
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
		"rqlite://" + host + "?insecure=true",
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
// top-level "node" key must not be confirmed as rqlite, and must
// step aside after a single request. (The hit count alone can't
// distinguish "no HTTPS fallback" from "fallback attempted but the
// TLS handshake failed before reaching the handler"; what it pins is
// the single-request step-aside.)
func TestDetectConnParams_NonRqliteJSON(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(countingHandler(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"8","status":"ok"}`))
		}), &hits))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	params, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://"+server.Listener.Addr().String()), nil)
	require.NoError(t, err)
	require.Nil(t, params, "non-rqlite JSON server: step aside")
	require.Equal(t, int64(1), hits.Load(), "exactly one probe request")
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

// TestDetectConnParams_HangingEndpointBounded pins two behaviors: the
// probe client's timeout comes from conn.open-timeout (a mutation
// passing zero or ignoring src.Options hangs this test), and a
// client-timeout failure is not a TLS signal (step aside, nil).
func TestDetectConnParams_HangingEndpointBounded(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(
		func(http.ResponseWriter, *http.Request) { <-block }))
	t.Cleanup(func() { close(block); server.Close() })

	src := detectTestSrc("rqlite://" + server.Listener.Addr().String())
	src.Options = options.Options{
		// Duration.Get requires a typed time.Duration value; a raw
		// "150ms" string would silently fall back to the default.
		driver.OptConnOpenTimeout.Key(): 150 * time.Millisecond,
	}

	start := time.Now()
	params, err := (&driveri{}).detectConnParams(context.Background(), src, nil)
	require.NoError(t, err)
	require.Nil(t, params, "timeout is not a TLS signal: step aside")
	require.Less(t, time.Since(start), 5*time.Second,
		"probe must be bounded by conn.open-timeout, not hang")
}

// TestDetectConnParams_RedirectNotFollowedToBackend pins that the
// probe never issues requests to a redirect target: a followed
// redirect would leak the probe (and its Authorization header) to
// the target. The front redirects to a separate TLS backend on the
// same hostname (different port; the classifier compares hostname
// only); with CheckRedirect intact the backend sees zero requests.
// Step 1 classifies the same-host https redirect as a TLS signal;
// step 2 then probes https against the FRONT's port (never the
// backend's), fails the handshake against the plain listener, and
// steps aside with nil params.
func TestDetectConnParams_RedirectNotFollowedToBackend(t *testing.T) {
	var backendHits atomic.Int64
	var host string
	backend := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			backendHits.Add(1)
			newRqliteMockHandler(&host).ServeHTTP(w, r)
		}))
	t.Cleanup(backend.Close)

	front := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, backend.URL+r.URL.Path, http.StatusMovedPermanently)
		}))
	t.Cleanup(front.Close)
	host = front.Listener.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	d := &driveri{}
	// The injected transport trusts the backend cert, so if the
	// client followed the redirect, the backend WOULD answer.
	params, err := d.detectConnParams(ctx,
		detectTestSrc("rqlite://"+host), backend.Client().Transport)
	require.NoError(t, err)
	require.Nil(t, params, "https probe of the plain front must step aside")
	require.Equal(t, int64(0), backendHits.Load(),
		"probe must never request the redirect target")
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

// TestDetectConnParams_BasicAuthPasswordOnly pins that the
// password-only userinfo form (rqlite://:pw@host) also produces an
// Authorization header, matching what gorqlite sends at connection
// time. Without this, an auth-gated /status on a TLS endpoint would
// 401 the probe and detection would silently step aside.
func TestDetectConnParams_BasicAuthPasswordOnly(t *testing.T) {
	var host string
	var gotAuth atomic.Bool
	inner := newRqliteMockHandler(&host)
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if u, p, ok := r.BasicAuth(); ok && u == "" && p == "pw" {
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
		detectTestSrc("rqlite://:pw@"+host), nil)
	require.NoError(t, err)
	require.True(t, gotAuth.Load(),
		"password-only userinfo must still produce an Authorization header")
}
