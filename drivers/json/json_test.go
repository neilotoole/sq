package json_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/json"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
)

var _ source.TypeDetectorFunc = json.DetectJSON
var _ source.TypeDetectorFunc = json.DetectJSONA
var _ source.TypeDetectorFunc = json.DetectJSONL

func TestTypeDetectorFunc(t *testing.T) {
	testCases := []struct {
		fn        source.TypeDetectorFunc
		fpath     string
		wantType  source.Type
		wantScore float64
		wantErr   bool
	}{
		{fn: json.DetectJSONA, fpath: "testdata/actor.jsona", wantType: json.TypeJSONA, wantScore: 1.0, wantErr: false},
		{fn: json.DetectJSONA, fpath: "testdata/actor.json", wantErr: true},
		{fn: json.DetectJSONA, fpath: "testdata/actor.jsonl", wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(testh.TName(tc.wantErr, tc.fpath), func(t *testing.T) {
			rdrs := testh.ReadersFor(tc.fpath)
			t.Cleanup(func() { require.NoError(t, rdrs.Close()) })

			gotType, gotScore, gotErr := tc.fn(context.Background(), rdrs)

			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.wantType, gotType)
			require.Equal(t, tc.wantScore, gotScore)
		})
	}
}
