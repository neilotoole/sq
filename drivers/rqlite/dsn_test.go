package rqlite

import (
	"testing"

	"github.com/stretchr/testify/require"
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
		{
			name:    "rqlites:// legacy still maps to https",
			loc:     "rqlites://host:4001",
			wantDSN: "https://host:4001",
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
	_, _, err := dsnFromLocation("rqlite://host:4001?tls=maybe")
	require.Error(t, err)
	require.Contains(t, err.Error(), `tls must be "true" or "false"`)
}
