package clickhouse

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
		{raw: "25.3.2.39", want: "v25.3.2"}, // four-part; regex caps at three
		{raw: "24.8.4.13", want: "v24.8.4"},
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
