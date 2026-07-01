package libsq

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
)

// executeCopyTasksFanIn copies each join-participant table into the join DB
// using a fan-in strategy: the source reads run concurrently while a single
// writer drains them into the join DB one table at a time. Because only one
// write transaction is ever open, the copies don't contend on the join DB's
// write lock (the "database is locked" failure of gh975), yet the source reads
// still overlap (#995), unlike the fully-serialized fix.
//
// Reads from the same source are serialized (see fanInReaders); reads from
// different sources overlap, which is where the cross-source win comes from.
//
// It is used when the join DB is single-writer (SQLite); a multi-writer joindb
// uses the concurrent fused path instead (see pipeline.executeTasks).
func executeCopyTasksFanIn(ctx context.Context, tasks []*joinCopyTask) error {
	buffers := make([]*copyBuffer, len(tasks))
	for i := range tasks {
		// Size from the destination (join DB) options, matching execCopyTable,
		// so tuning.record-buffer behaves consistently across the copy paths.
		// The actual recCh is allocated lazily in copyBuffer.Open, so only about
		// one buffer per concurrently-reading source is live at a time.
		bufSize := tuning.OptRecBufSize.Get(tasks[i].toGrip.Source().Options)
		buffers[i] = newCopyBuffer(bufSize)
	}

	readers := fanInReaders(tasks, buffers)

	return runCopyFanIn(
		ctx, readers, len(tasks),
		func(ctx context.Context, i int) error { return writeCopyTable(ctx, tasks[i], buffers[i]) },
	)
}

// fanInReaders builds the concurrent read side of the fan-in: one reader func
// per distinct source, each reading that source's tables sequentially (in task
// order) into their buffers.
//
// Same-source reads are serialized deliberately. All of a source's tables are
// read over that source's connection pool, and the single writer drains tables
// in task order; if two tables from one source were read concurrently and the
// pool were smaller than that (e.g. conn.max-open=1), a later table's reader
// could hold the only connection while blocked on its full buffer, starving the
// earlier table's reader that the writer is waiting on — deadlocking the query.
// Reading a source's tables in task order makes the writer's current table
// always the one holding that source's connection. It also bounds concurrency
// to the number of sources (typically two or three) rather than the table
// count. Same-source reads share one server/pool and gain little from
// concurrency anyway.
func fanInReaders(tasks []*joinCopyTask, buffers []*copyBuffer) []func(context.Context) error {
	handles := make([]string, len(tasks))
	for i, task := range tasks {
		handles[i] = task.fromGrip.Source().Handle
	}

	groups := groupTaskIndicesBySource(handles)
	readers := make([]func(context.Context) error, len(groups))
	for g, idxs := range groups {
		readers[g] = func(ctx context.Context) error {
			for _, i := range idxs {
				if err := readCopyTable(ctx, tasks[i], buffers[i]); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return readers
}

// groupTaskIndicesBySource groups task indices by their source handle,
// preserving first-seen source order and, within each group, task order.
func groupTaskIndicesBySource(handles []string) [][]int {
	var order []string
	groups := map[string][]int{}
	for i, h := range handles {
		if _, ok := groups[h]; !ok {
			order = append(order, h)
		}
		groups[h] = append(groups[h], i)
	}
	out := make([][]int, len(order))
	for i, h := range order {
		out[i] = groups[h]
	}
	return out
}

// runCopyFanIn is the concurrency skeleton of the fan-in copy: it runs every
// reader concurrently, and runs write for each of the nWrites tables from a
// single goroutine, in task order, so writes are serialized. A reader and the
// write for a given table coordinate out-of-band (via a shared copyBuffer);
// runCopyFanIn only owns the goroutine structure. Any error from a reader or a
// write cancels the shared context and is returned.
//
// readers and write are parameters (rather than inlined) so the orchestration
// can be tested without a database.
//
// The reader errgroup is intentionally uncapped. Concurrency is already bounded
// to the number of distinct sources by fanInReaders, and applying a hard
// OptErrgroupLimit here would risk deadlock: the single writer goroutine shares
// this errgroup, so a saturated limit could starve its slot (or a source the
// writer is waiting on). Overlapping the source reads is the point of the
// fan-in, so the distinct-source count is the intended bound. Safely bounding
// it further for very wide multi-source joins is tracked in #1009.
func runCopyFanIn(ctx context.Context, readers []func(context.Context) error,
	nWrites int, write func(ctx context.Context, i int) error,
) error {
	g, gCtx := errgroup.WithContext(ctx)

	for _, read := range readers {
		g.Go(func() error {
			return read(gCtx)
		})
	}

	g.Go(func() error {
		for i := range nWrites {
			if err := write(gCtx, i); err != nil {
				return err
			}
		}
		return nil
	})

	return g.Wait()
}

// newJoinDestTableHook returns a DBWriter pre-write hook that creates the join
// DB destination table destTbl from the incoming source record metadata. It is
// shared by the fan-in writer and execCopyTable so the two copy paths create
// join dest tables identically.
func newJoinDestTableHook(destTbl tablefq.T) DBWriterPreWriteHook {
	return func(ctx context.Context, recMeta record.Meta, destGrip driver.Grip, tx sqlz.DB) error {
		destTblDef := schema.NewTable(destTbl.Table, recMeta.Names(), recMeta.Kinds())
		if err := destGrip.SQLDriver().CreateTable(ctx, tx, destTblDef); err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destGrip.Source().Handle, destTbl)
		}
		return nil
	}
}

// copyBuffer is a [RecordWriter] that buffers records read from a source table
// into a bounded channel, decoupling the slow source read from the serialized
// write into the join DB. It performs no database work itself: the reader
// goroutine drives QuerySQL into it, and the writer goroutine drains it.
//
// The bounded recCh provides backpressure: a reader that outpaces the writer
// blocks once the buffer is full, rather than growing memory without bound.
//
// Field order is tuned for govet fieldalignment (the non-pointer fields, and
// the record.Meta slice whose len/cap are non-pointer, sit last).
type copyBuffer struct {
	// recCh is the bounded record buffer, allocated lazily in Open (when the
	// read actually starts) so only about one buffer per concurrently-reading
	// source is live at a time, not one per table. QuerySQL (the reader) sends
	// records to it and closes it when the read completes; writeCopyTable (the
	// writer) receives from it after metaReady.
	recCh chan record.Record

	// metaReady is closed once Open has run, signalling that meta and recCh are
	// valid so the writer can create the destination table and begin draining.
	metaReady chan struct{}

	// doneCh is closed by finish once the source read has completed. It is the
	// authoritative completion signal: recCh closing alone is ambiguous because
	// QuerySQL closes it on both success and failure.
	doneCh chan struct{}

	// errCh is handed to QuerySQL to satisfy the RecordWriter contract; it is
	// closed by finish (after QuerySQL has returned, so it does not disturb
	// QuerySQL's select loop). Cross-goroutine cancellation is handled via ctx
	// (the errgroup), and read success/failure via doneCh/readErr, so it never
	// carries a value.
	errCh chan error

	// readErr is the terminal read error (nil on success), set by finish before
	// doneCh closes; read by the writer only after doneCh is observed.
	readErr error

	// meta is the source record metadata, set in Open before metaReady closes.
	meta record.Meta

	// bufSize is the capacity applied to recCh when Open allocates it.
	bufSize int
}

func newCopyBuffer(bufSize int) *copyBuffer {
	return &copyBuffer{
		metaReady: make(chan struct{}),
		doneCh:    make(chan struct{}),
		errCh:     make(chan error),
		bufSize:   bufSize,
	}
}

// Open implements [RecordWriter]. It allocates the record buffer, records
// recMeta, and hands QuerySQL the buffer's record channel. cancelFn is unused:
// this sink is not driven via Wait (see the type doc).
func (b *copyBuffer) Open(_ context.Context, _ context.CancelFunc, recMeta record.Meta) (
	chan<- record.Record, <-chan error, error,
) {
	b.recCh = make(chan record.Record, b.bufSize)
	b.meta = recMeta
	close(b.metaReady)
	return b.recCh, b.errCh, nil
}

// Wait implements [RecordWriter]. It is present only to satisfy the interface
// for QuerySQL; the fan-in reader signals completion via finish instead, so
// this is not part of the normal flow.
func (b *copyBuffer) Wait() (int64, error) {
	<-b.doneCh
	return 0, b.readErr
}

// finish records the terminal read error (nil on success) and signals the
// writer that the source read is complete. It must be called exactly once,
// after QuerySQL returns. Closing errCh here (rather than leaving it open)
// satisfies the RecordWriter contract; QuerySQL has already returned, so it
// does not affect QuerySQL's select loop.
func (b *copyBuffer) finish(err error) {
	b.readErr = err
	close(b.errCh)
	close(b.doneCh)
}

// readCopyTable reads all rows of the task's source table into buf, running
// concurrently with the other readers. It always calls buf.finish so the
// writer is never left waiting, and returns any read error so the fan-in
// cancels promptly.
func readCopyTable(ctx context.Context, task *joinCopyTask, buf *copyBuffer) error {
	query := "SELECT * FROM " + task.fromTbl.Render(task.fromGrip.SQLDriver().Dialect().Enquote)
	err := QuerySQL(ctx, task.fromGrip, nil, buf, nil, query)
	buf.finish(err)
	if err != nil {
		return errz.Wrapf(err, "read %s.%s failed", task.fromGrip.Source().Handle, task.fromTbl)
	}
	return nil
}

// writeCopyTable drains buf into the join DB destination table via a DBWriter,
// running from the single writer goroutine. The commit is gated on the source
// read having succeeded: if the read failed (even after emitting some records),
// the destination transaction is rolled back rather than committing a partial
// copy.
func writeCopyTable(ctx context.Context, task *joinCopyTask, buf *copyBuffer) error {
	// Wait until the source read publishes its record metadata (needed to
	// create the destination table). A read that fails before producing any
	// metadata never closes metaReady, so also watch doneCh (and ctx) to avoid
	// blocking forever in that case.
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-buf.metaReady:
	case <-buf.doneCh:
	}

	// metaReady closing is the authoritative "metadata available" signal: it
	// always closes (in Open) before doneCh (in finish) on a successful read,
	// so a fast read whose doneCh won the race above is still handled here.
	// Only if metaReady is not closed did the read finish without producing
	// metadata, i.e. it failed before the first column-type fetch.
	select {
	case <-buf.metaReady:
	default:
		<-buf.doneCh
		if buf.readErr != nil {
			return errz.Wrapf(buf.readErr, "read %s.%s failed",
				task.fromGrip.Source().Handle, task.fromTbl)
		}
		return nil
	}

	// copyBuffer already provides the pre-writer buffering, so the DBWriter's
	// own record channel is unbuffered (recChSize 0) to avoid a redundant
	// second buffer of the active table's records.
	inserter := NewDBWriter("Copy records", task.toGrip, task.toTbl.Table, 0,
		newJoinDestTableHook(task.toTbl))

	// wCancel drives a rollback of the inserter's tx: DBWriter watches its ctx
	// and rolls back when it's cancelled.
	wCtx, wCancel := context.WithCancel(ctx)
	destCh, dbErrCh, err := inserter.Open(wCtx, wCancel, buf.meta)
	if err != nil {
		wCancel()
		return errz.Wrapf(err, "insert %s.%s failed", task.toGrip.Source().Handle, task.toTbl)
	}

	// Forward buffered records to the inserter until the read completes (recCh
	// closes) or something fails.
	if err = forwardRecords(wCtx, buf.recCh, destCh, dbErrCh); err != nil {
		wCancel()
		_, _ = inserter.Wait()
		return errz.Wrapf(err, "insert %s.%s failed", task.toGrip.Source().Handle, task.toTbl)
	}

	// recCh drained. Confirm the source read actually succeeded before
	// committing: QuerySQL closes recCh on failure too, so a closed channel
	// alone does not mean the copy is complete.
	<-buf.doneCh
	if buf.readErr != nil {
		wCancel() // roll back rather than commit a partial copy
		_, _ = inserter.Wait()
		return errz.Wrapf(buf.readErr, "read %s.%s failed", task.fromGrip.Source().Handle, task.fromTbl)
	}

	close(destCh) // signal the inserter to commit
	affected, err := inserter.Wait()
	if err != nil {
		return errz.Wrapf(err, "insert %s.%s failed", task.toGrip.Source().Handle, task.toTbl)
	}

	lg.FromContext(ctx).Debug("Copied rows to dest", lga.Count, affected,
		lga.From, fmt.Sprintf("%s.%s", task.fromGrip.Source().Handle, task.fromTbl),
		lga.To, fmt.Sprintf("%s.%s", task.toGrip.Source().Handle, task.toTbl))
	return nil
}

// forwardRecords copies records from src to dst until src is closed (the read
// completed), returning nil. It returns early with an error if ctx is
// cancelled or the destination writer reports an error on dbErrCh.
func forwardRecords(ctx context.Context, src <-chan record.Record, dst chan<- record.Record,
	dbErrCh <-chan error,
) error {
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case err := <-dbErrCh:
			return err
		case rec, ok := <-src:
			if !ok {
				return nil // src closed: read complete
			}
			select {
			case <-ctx.Done():
				return context.Cause(ctx)
			case err := <-dbErrCh:
				return err
			case dst <- rec:
			}
		}
	}
}
