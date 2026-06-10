package rqlite

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
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
