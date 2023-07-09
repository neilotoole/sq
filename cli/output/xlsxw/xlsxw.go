// Package xlsxw implements output writers for Microsoft Excel.
package xlsxw

import (
	"io"
	"time"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
)

type recordWriter struct {
	recMeta record.Meta
	pr      *output.Printing
	out     io.Writer
	header  bool
	xfile   *xlsx.File
	sheet   *xlsx.Sheet
}

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns an output.RecordWriter instance for XLSX.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, header: pr.ShowHeader}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	w.xfile = xlsx.NewFile()

	sheet, err := w.xfile.AddSheet("data")
	if err != nil {
		return errz.Wrap(err, "unable to create XLSX sheet")
	}

	w.sheet = sheet

	if w.header {
		headerRow := w.sheet.AddRow()

		for _, colName := range w.recMeta.MungedNames() {
			cell := headerRow.AddCell()
			cell.SetString(colName)
		}
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	err := w.xfile.Write(w.out)
	if err != nil {
		return errz.Wrap(err, "unable to write XLSX")
	}
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []record.Record) error {
	for _, rec := range recs {
		row := w.sheet.AddRow()

		for i, val := range rec {
			cell := row.AddCell()

			switch val := val.(type) {
			case nil:
			case []byte:
				cell.SetValue(val)
			case string:
				cell.SetString(val)
			case bool:
				cell.SetBool(val)
			case int64:
				cell.SetInt64(val)
			case float64:
				cell.SetFloat(val)
			case time.Time:
				switch w.recMeta[i].Kind() { //nolint:exhaustive
				default:
					cell.SetDateTime(val)
				case kind.Date:
					cell.SetDate(val)
				case kind.Time:
					// TODO: Maybe there's a way of setting a specific
					//  time (as opposed to date or datetime) value, but
					//  for now we just use a string.
					cell.SetValue(w.pr.FormatTime(val))
				}
			default:
				// should never happen
				cell.SetValue(val)
			}
		}
	}

	return nil
}
