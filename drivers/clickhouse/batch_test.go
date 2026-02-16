package clickhouse_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestBatchInsert_MultiBatch tests batch insert across multiple batch
// boundaries. With MaxBatchValues=10000 and TblPayment having 16049 rows,
// this produces 2 batches (10000 + 6049), exercising the batch boundary
// crossing logic in the ClickHouse native batch API.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestBatchInsert_MultiBatch(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	// Create an empty copy of the payment table.
	tblName := th.CopyTable(true, src, tablefq.From(sakila.TblPayment), tablefq.T{}, false)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Get all payment records (16049 rows, 7 columns).
	recMeta, recs := testh.RecordsFromTbl(t, sakila.CH, sakila.TblPayment)
	require.Equal(t, sakila.TblPaymentCount, len(recs),
		"expected %d payment records", sakila.TblPaymentCount)

	bi, err := drvr.NewBatchInsert(
		th.Context,
		"Insert payment records",
		conn,
		src,
		tblName,
		recMeta.Names(),
	)
	require.NoError(t, err)

	for _, rec := range recs {
		err = bi.Munge(rec)
		require.NoError(t, err)

		select {
		case <-th.Context.Done():
			close(bi.RecordCh)
			t.Fatal(th.Context.Err())
		case err = <-bi.ErrCh:
			close(bi.RecordCh)
			t.Fatal(err)
		case bi.RecordCh <- rec:
		}
	}
	close(bi.RecordCh)

	err = <-bi.ErrCh
	require.Nil(t, err)
	require.Equal(t, int64(sakila.TblPaymentCount), bi.Written())

	// Note: conn is closed by t.Cleanup registered above.

	// Verify the table has the expected row count.
	sink, err := th.QuerySQL(src, nil, "SELECT COUNT(*) FROM "+tblName)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, int64(sakila.TblPaymentCount), sink.Recs[0][0])
}

// TestBatchInsert_ContextCancel tests graceful cancellation of an in-progress
// batch insert. The test starts a batch insert, sends a few records, then
// cancels the context. The error channel should yield an error wrapping
// context.Canceled.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestBatchInsert_ContextCancel(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	recMeta, recs := testh.RecordsFromTbl(t, sakila.CH, sakila.TblActor)

	ctx, cancel := context.WithCancel(th.Context)
	defer cancel()

	bi, err := drvr.NewBatchInsert(
		ctx,
		"Insert actor records",
		conn,
		src,
		tblName,
		recMeta.Names(),
	)
	require.NoError(t, err)

	// Send a few records before cancelling.
	sendCount := min(5, len(recs))
	for i := range sendCount {
		err = bi.Munge(recs[i])
		require.NoError(t, err)

		select {
		case err = <-bi.ErrCh:
			t.Fatalf("unexpected error before cancel: %v", err)
		case bi.RecordCh <- recs[i]:
		}
	}

	// Cancel the context; do NOT close RecordCh.
	cancel()

	// The error channel should yield context.Canceled.
	err = <-bi.ErrCh
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled),
		"expected context.Canceled, got: %v", err)
}

// TestBatchInsert_Empty tests that a batch insert with zero records completes
// without error. The RecordCh is closed immediately after creation.
//
// This test requires a live ClickHouse instance and is skipped in short mode.
func TestBatchInsert_Empty(t *testing.T) {
	tu.SkipShort(t, true)

	th, src, drvr, _, db := testh.NewWith(t, sakila.CH)

	tblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	conn, err := db.Conn(th.Context)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	recMeta, _ := testh.RecordsFromTbl(t, sakila.CH, sakila.TblActor)

	bi, err := drvr.NewBatchInsert(
		th.Context,
		"Insert zero records",
		conn,
		src,
		tblName,
		recMeta.Names(),
	)
	require.NoError(t, err)

	// Immediately close the channel to signal no records.
	close(bi.RecordCh)

	err = <-bi.ErrCh
	require.Nil(t, err)
	require.Equal(t, int64(0), bi.Written())

	require.NoError(t, conn.Close())

	// Verify the table remains empty.
	sink, err := th.QuerySQL(src, nil, "SELECT COUNT(*) FROM "+tblName)
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
	require.Equal(t, int64(0), sink.Recs[0][0])
}
