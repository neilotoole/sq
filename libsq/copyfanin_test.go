package libsq_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestWriteCopyTable_readFailureRollsBack verifies the commit gate of the join
// copy fan-in (gh975/#995): when a source read fails after already emitting
// some records, the destination write must be rolled back rather than
// committing a partial copy. This matters because QuerySQL closes the record
// channel on failure too, so "channel closed" alone cannot signal success.
func TestWriteCopyTable_readFailureRollsBack(t *testing.T) {
	th := testh.New(t)
	ctx := th.Context
	src := th.Source(sakila.SL3)
	srcGrip := th.Open(src)

	// Grab real record metadata and a few records to feed the write side.
	qsink, err := th.QuerySQL(src, nil, "SELECT * FROM actor LIMIT 5")
	require.NoError(t, err)
	require.NotEmpty(t, qsink.Recs)

	// A fresh single-writer SQLite join DB as the copy destination.
	destGrip, err := th.Grips().OpenJoin(ctx, src)
	require.NoError(t, err)

	buf := libsq.NewCopyBuffer(1024)
	task := libsq.NewJoinCopyTask(srcGrip, tablefq.New("actor"), destGrip, tablefq.New("actor_copy"))

	// Simulate a source read that emits records and then fails — exactly what
	// readCopyTable does when QuerySQL errors after streaming some rows.
	go func() {
		recCh, _, openErr := buf.Open(ctx, func() {}, qsink.RecMeta)
		assert.NoError(t, openErr)
		for _, rec := range qsink.Recs {
			recCh <- rec
		}
		close(recCh)
		buf.FinishRead(errz.New("simulated read failure"))
	}()

	err = libsq.WriteCopyTable(ctx, task, buf)
	require.Error(t, err)
	require.ErrorContains(t, err, "simulated read failure")

	// The partial copy must have been rolled back: the destination table must
	// not exist, since its CREATE TABLE ran inside the rolled-back tx.
	db, err := destGrip.DB(ctx)
	require.NoError(t, err)
	exists, err := destGrip.SQLDriver().TableExists(ctx, db, "actor_copy")
	require.NoError(t, err)
	require.False(t, exists, "a partial copy must be rolled back, not committed")
}
