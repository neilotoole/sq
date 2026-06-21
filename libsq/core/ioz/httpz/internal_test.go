package httpz

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// White-box tests for unexported helpers that have a branch only reachable when
// a transport or TLS config already exists (which NewClient's normal flow never
// produces, since it starts from a freshly cloned http.DefaultTransport).

func TestBaseTransport(t *testing.T) {
	// Nil transport: clone http.DefaultTransport.
	tr := baseTransport(&http.Client{})
	require.NotNil(t, tr)

	// Non-nil transport: clone that transport (not http.DefaultTransport).
	custom := &http.Transport{MaxIdleConns: 7}
	got := baseTransport(&http.Client{Transport: custom})
	require.Equal(t, 7, got.MaxIdleConns)
	require.NotSame(t, custom, got, "the transport must be cloned, not shared")
}

func TestMinTLSVersion_apply(t *testing.T) {
	// Nil config: a fresh config is created with the minimum version.
	tr1 := &http.Transport{}
	minTLSVersion(tls.VersionTLS12).apply(tr1)
	require.NotNil(t, tr1.TLSClientConfig)
	require.Equal(t, uint16(tls.VersionTLS12), tr1.TLSClientConfig.MinVersion)

	// Existing config: it is cloned, MinVersion is set, and other fields are
	// preserved; the original config is left untouched.
	existing := &tls.Config{ServerName: "example.com", NextProtos: []string{"h2"}}
	tr2 := &http.Transport{TLSClientConfig: existing}
	minTLSVersion(tls.VersionTLS13).apply(tr2)
	require.Equal(t, uint16(tls.VersionTLS13), tr2.TLSClientConfig.MinVersion)
	require.Equal(t, "example.com", tr2.TLSClientConfig.ServerName)
	require.Equal(t, []string{"h2"}, tr2.TLSClientConfig.NextProtos)
	require.NotSame(t, existing, tr2.TLSClientConfig, "config must be cloned")
	require.Zero(t, existing.MinVersion, "the original config must not be mutated")
}

func TestOptInsecureSkipVerify_apply(t *testing.T) {
	// Nil config: a fresh config is created and the flag set.
	tr1 := &http.Transport{}
	OptInsecureSkipVerify(true).apply(tr1)
	require.NotNil(t, tr1.TLSClientConfig)
	require.True(t, tr1.TLSClientConfig.InsecureSkipVerify)

	// Existing config: the flag is set and other fields preserved.
	tr2 := &http.Transport{TLSClientConfig: &tls.Config{ServerName: "example.com"}}
	OptInsecureSkipVerify(true).apply(tr2)
	require.True(t, tr2.TLSClientConfig.InsecureSkipVerify)
	require.Equal(t, "example.com", tr2.TLSClientConfig.ServerName)
}
