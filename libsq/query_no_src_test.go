package libsq_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
)

func TestQuery_no_source(t *testing.T) {
	testCases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"1+2", "SELECT 1+2", false},
		{"(1+ 2) * 3", "SELECT (1+2)*3", false},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			t.Logf("\nquery: %s\n want: %s", tc.in, tc.want)
			th := testh.New(t)
			coll := th.NewCollection()
			dbases := th.Databases()

			qc := &libsq.QueryContext{
				Collection:      coll,
				DBOpener:        dbases,
				JoinDBOpener:    dbases,
				ScratchDBOpener: dbases,
			}

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, qc, tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			t.Log(gotSQL)
			assert.Equal(t, tc.want, gotSQL)
		})
	}
}
