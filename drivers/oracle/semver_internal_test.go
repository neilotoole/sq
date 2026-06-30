package oracle

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
		{raw: "23.26.1.0.0", want: "v23.26.1"}, // five-part; regex caps at three
		{raw: "19.0.0.0.0", want: "v19.0.0"},
		{raw: "23.5.0.24.07", want: "v23.5.0"},
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
