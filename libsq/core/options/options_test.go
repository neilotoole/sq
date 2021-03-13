package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const spatialite = "spatialite"

func TestGetBool(t *testing.T) {
	testCases := []struct {
		in      string
		name    string
		wantVal bool
		wantOK  bool
	}{
		{in: "header=true&spatialite=true", name: spatialite, wantVal: true, wantOK: true},
		{in: "spatialite=false", name: spatialite, wantVal: false, wantOK: true},
		{in: "header=true", name: spatialite, wantVal: false, wantOK: false},
		{in: "", name: spatialite, wantVal: false, wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			opts, gotErr := Parse(tc.in)
			require.NoError(t, gotErr)

			gotVal, gotOK, gotErr := opts.GetBool(tc.name)
			require.NoError(t, gotErr)
			require.Equal(t, tc.wantOK, gotOK)
			require.Equal(t, tc.wantVal, gotVal)
		})
	}

}
