package htmlw_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestRecordWriter(t *testing.T) {
	testCases := []struct {
		name     string
		numRecs  int
		fixtPath string
	}{
		{name: "actor_0", numRecs: 0, fixtPath: "testdata/actor_0_rows.html"},
		{name: "actor_3", numRecs: 3, fixtPath: "testdata/actor_3_rows.html"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			recMeta, recs := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
			recs = recs[0:tc.numRecs]

			buf := &bytes.Buffer{}
			w := htmlw.NewRecordWriter(buf)
			require.NoError(t, w.Open(recMeta))

			require.NoError(t, w.WriteRecords(recs))
			require.NoError(t, w.Close())

			want, err := os.ReadFile(tc.fixtPath)
			require.NoError(t, err)
			require.Equal(t, string(want), buf.String())
		})
	}
}
