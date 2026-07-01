package libsq_test

import (
	"testing"
	"time"

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

// TestExecuteCopyTasksFanIn_sameSourceNoDeadlock is a regression test for the
// gh975/#995 review finding: two tables copied from the same source, whose
// connection pool is smaller than the number of those tables (here
// conn.max-open=1), must not deadlock. The fan-in serializes same-source reads
// so the single in-order writer's current table always holds the source's lone
// connection. With unbounded per-table reads, a later table's reader could grab
// the only connection, fill its buffer, and block — starving the reader the
// writer is waiting on, hanging the query forever.
func TestExecuteCopyTasksFanIn_sameSourceNoDeadlock(t *testing.T) {
	th := testh.New(t)
	ctx := th.Context

	src := th.Source(sakila.SL3)
	srcGrip := th.Open(src)

	// Force the source's connection pool to a single connection, so concurrent
	// same-source reads would contend (and, unserialized, deadlock).
	srcDB, err := srcGrip.DB(ctx)
	require.NoError(t, err)
	srcDB.SetMaxOpenConns(1)

	destGrip, err := th.Grips().OpenJoin(ctx, src)
	require.NoError(t, err)

	// Two large same-source tables, each with far more rows than the record
	// buffer (default 1024), so a reader that gets ahead blocks on backpressure
	// while holding the connection — the deadlock precondition.
	tasks := []*libsq.JoinCopyTask{
		libsq.NewJoinCopyTask(srcGrip, tablefq.New("rental"), destGrip, tablefq.New("rental")),
		libsq.NewJoinCopyTask(srcGrip, tablefq.New("payment"), destGrip, tablefq.New("payment")),
	}

	// Guard against a hang: fail fast rather than blocking the suite.
	done := make(chan error, 1)
	go func() { done <- libsq.ExecuteCopyTasksFanIn(ctx, tasks) }()
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("fan-in copy deadlocked (conn.max-open=1, two same-source tables)")
	}

	// Both tables must have copied fully (not a partial/short copy).
	destDB, err := destGrip.DB(ctx)
	require.NoError(t, err)
	for _, tbl := range []string{"rental", "payment"} {
		var srcN, destN int64
		require.NoError(t, srcDB.QueryRowContext(ctx, "SELECT count(*) FROM "+tbl).Scan(&srcN))
		require.NoError(t, destDB.QueryRowContext(ctx, "SELECT count(*) FROM "+tbl).Scan(&destN))
		require.Equal(t, srcN, destN, "table %s copied incompletely", tbl)
		require.Greater(t, destN, int64(1024),
			"table %s must exceed the record buffer to exercise backpressure", tbl)
	}
}
