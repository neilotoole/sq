package htmlw_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
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
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recMeta, recs := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
			recs = recs[0:tc.numRecs]

			buf := &bytes.Buffer{}
			pr := output.NewPrinting()
			w := htmlw.NewRecordWriter(buf, pr)
			require.NoError(t, w.Open(ctx, recMeta))

			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))

			want, err := os.ReadFile(tc.fixtPath)
			require.NoError(t, err)
			require.Equal(t, string(want), buf.String())
		})
	}
}
