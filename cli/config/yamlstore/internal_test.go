package yamlstore

import (
	"strings"
	"testing"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
)

// Export for testing.
var (
	LoadVersion = loadVersionFromFile
)

func Test_getConfigDirFromFlag(t *testing.T) {
	testCases := []struct {
		in      []string
		want    string
		wantOK  bool
		wantErr bool
	}{
		{in: nil, wantOK: false},
		{in: []string{}, wantOK: false},
		{in: []string{"sq", "--" + flag.Config + "=/home"}, wantOK: true, want: "/home"},
		{in: []string{"sq", "--" + flag.Config, "/home"}, wantOK: true, want: "/home"},
		{in: []string{"sq", "some", "args"}, wantOK: false, want: ""},
		{in: []string{"sq", "--some=value", "args"}, wantOK: false, want: ""},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.in, " ")), func(t *testing.T) {
			got, gotOK, gotErr := getConfigDirFromFlag(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			if !tc.wantOK {
				require.False(t, gotOK)
				require.Empty(t, got)
				return
			}

			require.True(t, tc.wantOK)
			require.Equal(t, tc.want, got)
		})
	}
}
