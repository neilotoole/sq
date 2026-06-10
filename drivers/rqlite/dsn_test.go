package rqlite

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

func TestDsnFromLocation_TLSParam(t *testing.T) {
	testCases := []struct {
		name    string
		loc     string
		wantDSN string
		wantTLS bool
	}{
		{
			name:    "no tls param defaults to http",
			loc:     "rqlite://host:4001",
			wantDSN: "http://host:4001",
			wantTLS: false,
		},
		{
			name:    "tls=true rewrites to https and strips param",
			loc:     "rqlite://host:4001?tls=true",
			wantDSN: "https://host:4001",
			wantTLS: true,
		},
		{
			name:    "tls=false stays http and strips param",
			loc:     "rqlite://host:4001?tls=false",
			wantDSN: "http://host:4001",
			wantTLS: false,
		},
		{
			name:    "tls=true preserves other query params",
			loc:     "rqlite://host:4001?level=strong&tls=true",
			wantDSN: "https://host:4001?level=strong",
			wantTLS: true,
		},
		{
			name:    "tls=true preserves credentials",
			loc:     "rqlite://alice:pw@host:4001?tls=true",
			wantDSN: "https://alice:pw@host:4001",
			wantTLS: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn, opts, err := dsnFromLocation(tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.wantDSN, dsn)
			require.Equal(t, tc.wantTLS, opts.tls)
		})
	}
}

func TestDsnFromLocation_TLSInvalidValue(t *testing.T) {
	testCases := []struct {
		name string
		loc  string
	}{
		{name: "junk value", loc: "rqlite://host:4001?tls=maybe"},
		{name: "empty value", loc: "rqlite://host:4001?tls="},
		{name: "bare key", loc: "rqlite://host:4001?tls"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := dsnFromLocation(tc.loc)
			require.Error(t, err)
			require.Contains(t, err.Error(), `tls must be "true" or "false"`)
		})
	}
}

func TestDsnFromLocation_InsecureParam(t *testing.T) {
	testCases := []struct {
		name         string
		loc          string
		wantDSN      string
		wantTLS      bool
		wantInsecure bool
	}{
		{
			name:         "no insecure param defaults to false",
			loc:          "rqlite://host:4001?tls=true",
			wantDSN:      "https://host:4001",
			wantTLS:      true,
			wantInsecure: false,
		},
		{
			name:         "insecure=true with tls=true",
			loc:          "rqlite://host:4001?tls=true&insecure=true",
			wantDSN:      "https://host:4001",
			wantTLS:      true,
			wantInsecure: true,
		},
		{
			name:         "insecure=false with tls=true is a noop but allowed",
			loc:          "rqlite://host:4001?tls=true&insecure=false",
			wantDSN:      "https://host:4001",
			wantTLS:      true,
			wantInsecure: false,
		},
		{
			name:         "both params stripped from DSN",
			loc:          "rqlite://host:4001?level=strong&tls=true&insecure=true",
			wantDSN:      "https://host:4001?level=strong",
			wantTLS:      true,
			wantInsecure: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dsn, opts, err := dsnFromLocation(tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.wantDSN, dsn)
			require.Equal(t, tc.wantTLS, opts.tls)
			require.Equal(t, tc.wantInsecure, opts.insecure)
		})
	}
}

func TestDsnFromLocation_InsecureContradictions(t *testing.T) {
	testCases := []struct {
		name        string
		loc         string
		wantErrFrag string
	}{
		{
			name:        "insecure without tls",
			loc:         "rqlite://host:4001?insecure=true",
			wantErrFrag: "insecure has no effect without tls=true",
		},
		{
			name:        "insecure with tls=false",
			loc:         "rqlite://host:4001?tls=false&insecure=true",
			wantErrFrag: "insecure has no effect without tls=true",
		},
		{
			name:        "insecure with invalid value",
			loc:         "rqlite://host:4001?tls=true&insecure=maybe",
			wantErrFrag: `insecure must be "true" or "false"`,
		},
		{
			name:        "insecure with empty value",
			loc:         "rqlite://host:4001?tls=true&insecure=",
			wantErrFrag: `insecure must be "true" or "false"`,
		},
		{
			name:        "bare insecure key",
			loc:         "rqlite://host:4001?tls=true&insecure",
			wantErrFrag: `insecure must be "true" or "false"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := dsnFromLocation(tc.loc)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErrFrag)
		})
	}
}

// TestDsnFromLocation_RejectsRqlitesScheme pins the removal of the
// legacy rqlites:// scheme: only rqlite:// (with ?tls=true for HTTPS)
// is accepted.
func TestDsnFromLocation_RejectsRqlitesScheme(t *testing.T) {
	_, _, err := dsnFromLocation("rqlites://host:4001")
	require.Error(t, err)
	require.Contains(t, err.Error(), `must start with "rqlite://"`)
}

func TestValidateSource_RejectsContradictions(t *testing.T) {
	d := &driveri{}
	src := &source.Source{
		Type:     drivertype.Rqlite,
		Location: "rqlite://h:4001?insecure=true",
	}
	_, err := d.ValidateSource(src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insecure has no effect without tls=true")
}

// TestValidateSource_AllowsPlaceholderLocation verifies that a
// ${keyring:...} placeholder location (minted by sq add --store
// keyring before validation runs) skips the URL grammar check, which
// runs at Open time on the resolved location instead.
func TestValidateSource_AllowsPlaceholderLocation(t *testing.T) {
	d := &driveri{}
	src := &source.Source{
		Type:     drivertype.Rqlite,
		Location: "${keyring:abc123}",
	}
	got, err := d.ValidateSource(src)
	require.NoError(t, err)
	require.Same(t, src, got)
}

func TestInsecureClientTimeout(t *testing.T) {
	// The gorqlite-native ?timeout param wins over the option default.
	got := insecureClientTimeout("https://host:4001?timeout=30", options.Options{})
	require.Equal(t, 30*time.Second, got)

	// No param: falls back to OptConnOpenTimeout's default.
	got = insecureClientTimeout("https://host:4001", options.Options{})
	require.Equal(t, driver.OptConnOpenTimeout.Get(options.Options{}), got)

	// Invalid param: falls back.
	got = insecureClientTimeout("https://host:4001?timeout=abc", options.Options{})
	require.Equal(t, driver.OptConnOpenTimeout.Get(options.Options{}), got)

	// Non-positive param: falls back.
	got = insecureClientTimeout("https://host:4001?timeout=0", options.Options{})
	require.Equal(t, driver.OptConnOpenTimeout.Get(options.Options{}), got)
}

func TestConnParams_HasTLSAndInsecure(t *testing.T) {
	d := &driveri{}
	params := d.ConnParams()
	require.Contains(t, params, "tls")
	require.ElementsMatch(t, []string{"true", "false"}, params["tls"])
	require.Contains(t, params, "insecure")
	require.ElementsMatch(t, []string{"true", "false"}, params["insecure"])
}
