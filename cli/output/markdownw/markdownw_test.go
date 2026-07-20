package markdownw_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/markdownw"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestRecordWriter(t *testing.T) {
	const (
		want0 = `| actor_id | first_name | last_name | last_update |
| --- | --- | --- | --- |
`
		want3 = `| actor_id | first_name | last_name | last_update |
| --- | --- | --- | --- |
| 1 | PENELOPE | GUINESS | 2006-02-15T04:34:33Z |
| 2 | NICK | WAHLBERG | 2006-02-15T04:34:33Z |
| 3 | ED | CHASE | 2006-02-15T04:34:33Z |
`
	)

	testCases := []struct {
		name    string
		numRecs int
		want    string
	}{
		{name: "actor_0", numRecs: 0, want: want0},
		{name: "actor_3", numRecs: 3, want: want3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			recMeta, recs := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
			recs = recs[0:tc.numRecs]

			buf := &bytes.Buffer{}
			w := markdownw.NewRecordWriter(buf, output.NewPrinting())
			require.NoError(t, w.Open(ctx, recMeta))

			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))
			require.Equal(t, tc.want, buf.String())
		})
	}
}
