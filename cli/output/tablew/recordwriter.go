package tablew

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// RecordWriter implements several of pkg out's writer interfaces.
type RecordWriter struct {
	tbl      *table
	recMeta  sqlz.RecordMeta
	rowCount int
}

func NewRecordWriter(out io.Writer, fm *output.Formatting, header bool) *RecordWriter {
	tbl := &table{out: out, fm: fm, header: header}
	w := &RecordWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

func (w *RecordWriter) Open(recMeta sqlz.RecordMeta) error {
	w.recMeta = recMeta
	return nil
}

func (w *RecordWriter) Flush() error {
	return nil
}

func (w *RecordWriter) Close() error {
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

func (w *RecordWriter) WriteRecords(recs []sqlz.Record) error {
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
