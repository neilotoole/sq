package tablew

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

type recordWriter struct {
	tbl      *table
	recMeta  sqlz.RecordMeta
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
func (w *recordWriter) Open(recMeta sqlz.RecordMeta) error {
	w.recMeta = recMeta
	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	if w.rowCount == 0 {
		// no data to write
		return nil
	}

	w.tbl.tblImpl.SetAutoWrapText(false)
	header := w.recMeta.Names()
	w.tbl.tblImpl.SetHeader(header)

	w.tbl.renderAll()
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []sqlz.Record) error {
	kinds := w.recMeta.Kinds()

	var tblRows [][]string
	for _, rec := range recs {
		tblRow := make([]string, len(rec))

		for i, val := range rec {
			tblRow[i] = w.tbl.renderResultCell(kinds[i], val)
		}

		tblRows = append(tblRows, tblRow)
		w.rowCount++
	}

	w.tbl.appendRows(tblRows)
	return nil
}
