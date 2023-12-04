package tablew

import (
	"context"
	"io"
	"sync"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/record"
)

type recordWriter struct {
	mu       sync.Mutex
	tbl      *table
	recMeta  record.Meta
	rowCount int
}

// NewRecordWriter returns a RecordWriter for text table output.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	tbl := &table{out: out, pr: pr, header: pr.ShowHeader}
	w := &recordWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(_ context.Context, recMeta record.Meta) error {
	w.recMeta = recMeta
	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush(context.Context) error {
	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close(ctx context.Context) error {
	if w.rowCount == 0 {
		// no data to write
		return nil
	}

	w.tbl.tblImpl.SetAutoWrapText(false)
	header := w.recMeta.MungedNames()
	w.tbl.tblImpl.SetHeader(header)

	return w.tbl.renderAll(ctx)
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
	}

	return w.tbl.appendRows(ctx, tblRows)
}
