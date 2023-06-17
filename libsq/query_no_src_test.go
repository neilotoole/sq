package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
)

func TestQueryNoSource(t *testing.T) {
	testCases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"1+2", "SELECT 1 + 2", false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			coll := testh.New(t).NewCollection()

			th := testh.New(t)
			dbases := th.Databases()

			qc := &libsq.QueryContext{
				Collection:   coll,
				DBOpener:     dbases,
				JoinDBOpener: dbases,
			}

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, qc, "1+2")
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, gotSQL)
			t.Log(gotSQL)
		})
	}
}
