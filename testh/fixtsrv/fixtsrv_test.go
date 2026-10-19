package fixtsrv_test

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/fixtsrv"
)

// TestBaseURL starts the server and checks it exports its URL in the envar,
// so test.sq.yml's ${env:SQ_TEST_FIXTURE_URL} placeholder can resolve.
func TestBaseURL(t *testing.T) {
	got := fixtsrv.BaseURL()
	require.NotEmpty(t, got)
	require.Equal(t, got, os.Getenv(fixtsrv.EnvFixtureURL))

	// The server must bind loopback. Check the parsed IP rather than matching
	// the string "127.0.0.1": httptest falls back to [::1] on a host with no
	// IPv4 loopback, which is still correct.
	u, err := url.Parse(got)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	ip := net.ParseIP(host)
	require.NotNil(t, ip, "host %q should be an IP literal", host)
	require.True(t, ip.IsLoopback(), "server must bind loopback, got %s", ip)

	// The server is a singleton: a second call returns the same URL.
	require.Equal(t, got, fixtsrv.BaseURL())
}

// TestURL_servesFixtures checks each fixture the tests actually fetch.
func TestURL_servesFixtures(t *testing.T) {
	for _, name := range []string{"actor.csv", "sakila.xlsx", "sakila_subset.xlsx"} {
		t.Run(name, func(t *testing.T) {
			resp, err := http.Get(fixtsrv.URL(name))
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			n, err := io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.Equal(t, fixtsrv.Size(name), int(n),
				"served byte count must match the file on disk")
		})
	}
}

// TestCacheControl is a regression guard, not a style check. Without a
// max-age the downloader's getFreshness leaves lifetime at zero and reports
// Stale, which breaks TestDownloader's Fresh assertion. See gh #1158.
func TestCacheControl(t *testing.T) {
	resp, err := http.Get(fixtsrv.URL("actor.csv"))
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()

	require.Equal(t, "public, max-age=300", resp.Header.Get("Cache-Control"))
}

// TestSize matches the file on disk, so a fixture change needs no edit here.
func TestSize(t *testing.T) {
	fi, err := os.Stat(fixtsrv.Dir() + "/actor.csv")
	require.NoError(t, err)
	require.Equal(t, int(fi.Size()), fixtsrv.Size("actor.csv"))
}
