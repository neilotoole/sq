package postgres

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "16.1", want: "v16.1.0"},                             // PG 10+ is two-part; Canonical pads
		{raw: "16.1 (Ubuntu 16.1-1.pgdg22.04+1)", want: "v16.1.0"}, // distro parenthetical after token
		{raw: "9.6.24", want: "v9.6.24"},                           // pre-10 three-part
		{raw: "not-a-version", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSemver(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
