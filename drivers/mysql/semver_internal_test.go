package mysql

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
		{raw: "8.0.36-0ubuntu0.22.04.1", want: "v8.0.36"},
		{raw: "5.7.44", want: "v5.7.44"},
		{raw: "5.7", want: "v5.7.0"},
		{raw: "8.4.0", want: "v8.4.0"},
		{raw: "5.5.5-10.6.4-MariaDB", want: "v10.6.4"},                     // MariaDB replication sentinel
		{raw: "10.11.2-MariaDB-1:10.11.2+maria~ubu2204", want: "v10.11.2"}, // modern MariaDB, no sentinel
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
