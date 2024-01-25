package raww_test

import (
	"bytes"
	"context"
	"image/gif"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/raww"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

func TestRecordWriter_TblActor(t *testing.T) {
	testCases := []struct {
		name    string
		numRecs int
		want    []byte
	}{
		{name: "actor_0", numRecs: 0, want: nil},
		{
			name: "actor_3", numRecs: 3,
			want: []byte("1PENELOPEGUINESS2020-06-11T02:50:54Z2NICKWAHLBERG2020-06-11T02:50:54Z3EDCHASE2020-06-11T02:50:54Z"),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			recMeta, recs := testh.RecordsFromTbl(t, sakila.SL3, sakila.TblActor)
			recs = recs[0:tc.numRecs]

			buf := &bytes.Buffer{}
			w := raww.NewRecordWriter(buf, output.NewPrinting())
			require.NoError(t, w.Open(ctx, recMeta))
			require.NoError(t, w.WriteRecords(ctx, recs))
			require.NoError(t, w.Close(ctx))
			require.Equal(t, tc.want, buf.Bytes())
		})
	}
}

func TestRecordWriter_TblBytes(t *testing.T) {
	th := testh.New(t, testh.OptNoLog())
	src := th.Source(testsrc.MiscDB)
	sink, err := th.QuerySQL(src, nil, "SELECT col_bytes FROM tbl_bytes WHERE col_name='gopher.gif'")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

	fBytes := proj.ReadFile(fixt.GopherPath)

	buf := &bytes.Buffer{}
	w := raww.NewRecordWriter(buf, output.NewPrinting())
	require.NoError(t, w.Open(th.Context, sink.RecMeta))
	require.NoError(t, w.WriteRecords(th.Context, sink.Recs))
	require.NoError(t, w.Close(th.Context))

	require.Equal(t, fBytes, buf.Bytes())
	_, err = gif.Decode(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
}
