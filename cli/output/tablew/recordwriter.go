package tablew

import (
	"context"
	"io"
	"sync"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
)

type recordWriter struct {
	tbl      *table
	bar      *progress.Bar
	recMeta  record.Meta
	rowCount int
	mu       sync.Mutex
}

// NewRecordWriter returns a RecordWriter for text table output.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	tbl := &table{out: out, pr: pr, header: pr.ShowHeader}
	w := &recordWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(ctx context.Context, recMeta record.Meta) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.recMeta = recMeta

	// We show a progress bar, because this writer batches all records and writes
	// them together at the end. A better-behaved writer would stream records
	// as they arrive (or at least batch them in smaller chunks). This will
	// probably be fixed at some point, but there's a bit of a catch. The table
	// determines the width of each column based on the widest value seen for that
	// column. So, if we stream records as they arrive, we can't know the maximum
	// width of each column until all records have been received. Thus,
	// periodically flushing the output may result in inconsistent bar widths for
	// subsequent batches. This is probably something that we'll have to live
	// with. After all, this writer is intended for human/interactive use, and
	// if the number of records is huge (triggering batching), then the user
	// really should be using a machine-readable output format instead.
	w.bar = progress.FromContext(ctx).NewUnitCounter("Preparing output", "rec", progress.OptMemUsage)

	return nil
}

// Flush implements output.RecordWriter. It's a no-op for this writer.
func (w *recordWriter) Flush(context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.bar != nil {
		w.bar.Stop()
	}
	if w.rowCount == 0 {
		// no data to write
		return nil
	}

	w.tbl.tblImpl.SetAutoWrapText(false)
	header := w.recMeta.MungedNames()
	w.tbl.tblImpl.SetHeader(header)

	return w.tbl.writeAll(ctx)
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	kinds := w.recMeta.Kinds()

	var tblRows [][]string
	for _, rec := range recs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		tblRow := make([]string, len(rec))

		for i, val := range rec {
			tblRow[i] = w.tbl.renderResultCell(kinds[i], val)
		}

		tblRows = append(tblRows, tblRow)
		w.rowCount++
		w.bar.Incr(1)
	}

	return w.tbl.appendRows(ctx, tblRows)
}
