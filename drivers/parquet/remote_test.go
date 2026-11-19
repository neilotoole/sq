package parquet_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
)

// newRemoteParquetServer returns an httptest server that serves the sakila
// parquet actor fixture at every path. Requests under /presigned/ emulate an
// S3 presigned GET URL: SigV4 signs the HTTP method, so S3 rejects a HEAD
// against a URL that was presigned for GET with 403 Forbidden. A server that
// answers GET (and Range GET) but not HEAD is the behavior sq must tolerate.
func newRemoteParquetServer(t *testing.T) *httptest.Server {
	t.Helper()

	body, err := os.ReadFile(proj.Abs(sakila.PathParquetActor))
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/presigned/") {
			if r.URL.Query().Get("X-Amz-Signature") == "" || r.Method != http.MethodGet {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Content-Type", "binary/octet-stream")
		http.ServeContent(w, r, "actor.parquet", time.Time{}, bytes.NewReader(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestRemote_CLI exercises the full CLI path (add, ping, inspect, query) for
// parquet sources served over HTTP. Driver-level tests that call Open directly
// bypass sq add's ping and inspect's SourceMetadata, which is where remote
// sources previously failed.
func TestRemote_CLI(t *testing.T) {
	srv := newRemoteParquetServer(t)

	testCases := []struct {
		name     string
		path     string
		wantName string
	}{
		{name: "plain", path: "/bucket/actor.parquet", wantName: "actor.parquet"},
		{
			name: "presigned",
			path: "/presigned/actor.parquet?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
				"&X-Amz-Expires=3600&X-Amz-Signature=abc123",
			wantName: "actor.parquet",
		},
		{name: "no_ext", path: "/download?id=42", wantName: "download"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			loc := srv.URL + tc.path
			const handle = "@remote"

			tr := testrun.New(ctx, t, nil)
			require.NoError(t, tr.Exec("add", loc, "--handle", handle))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("ping", handle))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("inspect", "-j", "--overview", handle))
			md := tr.BindMap()
			require.Equal(t, drivertype.Parquet.String(), md["driver"])
			require.Equal(t, loc, md["location"])
			require.Equal(t, tc.wantName, md["name"])
			require.NotContains(t, md, "size", "size of a remote parquet file is not known")

			tr = testrun.New(ctx, t, tr).Hush()
			require.NoError(t, tr.Exec("--csv", "--no-header", handle+".data | count"))
			require.Equal(t, [][]string{{"200"}}, tr.BindCSV())
		})
	}
}

// TestRemote_CLI_PingUnreachable verifies that ping reports a remote parquet
// location that can't be read.
func TestRemote_CLI_PingUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	const handle = "@remote"
	tr := testrun.New(ctx, t, nil)
	require.NoError(t, tr.Exec("add", "--skip-verify", "--driver", drivertype.Parquet.String(),
		srv.URL+"/missing.parquet", "--handle", handle))

	tr = testrun.New(ctx, t, tr)
	require.Error(t, tr.Exec("ping", handle))
}
