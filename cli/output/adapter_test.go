package output_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/testsrc"
)

var _ libsq.RecordWriter = (*output.RecordWriterAdapter)(nil)

func TestRecordWriterAdapter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		handle       string
		sqlQuery     string
		wantRowCount int
		wantColCount int
	}{
		{
			handle:       sakila.SL3,
			sqlQuery:     "SELECT * FROM actor",
			wantRowCount: sakila.TblActorCount,
			wantColCount: len(sakila.TblActorCols()),
		},
		{
			handle:       testsrc.CSVPersonBig,
			sqlQuery:     "SELECT * FROM data",
			wantRowCount: 10000,
			wantColCount: len(sakila.TblActorCols()),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(tc.handle)
			dbase := th.Open(src)

			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(sink)
			err := libsq.QuerySQL(th.Context, th.Log, dbase, recw, tc.sqlQuery)
			require.NoError(t, err)
			written, err := recw.Wait()
			require.NoError(t, err)
			require.Equal(t, int64(tc.wantRowCount), written)

			require.True(t, len(sink.Closed) == 1)
			require.Equal(t, tc.wantRowCount, len(sink.Recs))
			require.Equal(t, tc.wantColCount, len(sink.Recs[0]))
		})
	}
}

func TestRecordWriterAdapter_FlushAfterN(t *testing.T) {
	const writeRecCount = 200

	testCases := map[int]int{
		-1:                0,
		0:                 writeRecCount,
		1:                 writeRecCount,
		2:                 writeRecCount / 2,
		10:                writeRecCount / 10,
		100:               writeRecCount / 100,
		writeRecCount + 1: 0,
	}
	// Get some recMeta to feed to RecordWriter.Open.
	// In this case, the field is "actor_id", which is an int.
	recMeta := testh.NewRecordMeta([]string{"col_int"}, []kind.Kind{kind.KindInt})

	for flushAfterN, wantFlushed := range testCases {
		flushAfterN, wantFlushed := flushAfterN, wantFlushed

		t.Run(fmt.Sprintf("flustAfter_%d__wantFlushed_%d", flushAfterN, wantFlushed), func(t *testing.T) {
			t.Parallel()

			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(sink)

			recw.FlushAfterN = int64(flushAfterN)
			recw.FlushAfterDuration = -1 // not testing duration
			recCh, _, err := recw.Open(context.Background(), nil, recMeta)
			require.NoError(t, err)

			// Write some records
			for i := 0; i < writeRecCount; i++ {
				recCh <- []interface{}{1}
			}
			close(recCh)

			written, err := recw.Wait()
			require.NoError(t, err)
			require.Equal(t, int64(writeRecCount), written)
			require.Equal(t, writeRecCount, len(sink.Recs))

			require.Equal(t, wantFlushed, len(sink.Flushed))
		})
	}
}

func TestRecordWriterAdapter_FlushAfterDuration(t *testing.T) {
	// Don't run this as t.Parallel because it's timing sensitive.
	const (
		sleepTime     = time.Millisecond * 10
		writeRecCount = 10
	)

	testCases := []struct {
		flushAfter  time.Duration
		wantFlushed int
		assertFn    testh.AssertCompareFunc
	}{
		{flushAfter: -1, wantFlushed: 0, assertFn: require.Equal},
		{flushAfter: 0, wantFlushed: 0, assertFn: require.Equal},
		{flushAfter: 1, wantFlushed: 10, assertFn: require.GreaterOrEqual},
		{flushAfter: time.Millisecond * 20, wantFlushed: 2, assertFn: require.GreaterOrEqual},
		{flushAfter: time.Second, wantFlushed: 0, assertFn: require.Equal},
	}

	// Get some recMeta to feed to RecordWriter.Open.
	// In this case, the field is "actor_id", which is an int.
	recMeta := testh.NewRecordMeta([]string{"col_int"}, []kind.Kind{kind.KindInt})

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("flushAfter_%s__wantFlushed_%d", tc.flushAfter, tc.wantFlushed), func(t *testing.T) {
			t.Parallel()

			sink := &testh.RecordSink{}
			recw := output.NewRecordWriterAdapter(sink)

			recw.FlushAfterN = -1 // not testing FlushAfterN
			recw.FlushAfterDuration = tc.flushAfter

			recCh, _, err := recw.Open(context.Background(), nil, recMeta)
			require.NoError(t, err)

			// Write some records
			for i := 0; i < writeRecCount; i++ {
				recCh <- []interface{}{1}
				time.Sleep(sleepTime)
			}
			close(recCh)

			written, err := recw.Wait()
			require.NoError(t, err)
			require.Equal(t, int64(writeRecCount), written)
			require.Equal(t, writeRecCount, len(sink.Recs))

			tc.assertFn(t, len(sink.Flushed), tc.wantFlushed)
		})
	}
}
