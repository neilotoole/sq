// Package csvw implements writers for CSV.
package csvw

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
)

const (
	// Tab is the tab rune.
	Tab = '\t'

	// Comma is the comma rune.
	Comma = ','
)

// RecordWriter implements output.RecordWriter.
type RecordWriter struct {
	mu          sync.Mutex
	recMeta     record.Meta
	cw          *csv.Writer
	needsHeader bool
	pr          *output.Printing
}

var (
	_ output.NewRecordWriterFunc = NewCommaRecordWriter
	_ output.NewRecordWriterFunc = NewTabRecordWriter
)

// NewCommaRecordWriter returns writer instance that uses csvw.Comma.
func NewCommaRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return newRecordWriter(out, pr, Comma)
}

// NewTabRecordWriter returns writer instance that uses csvw.Comma.
func NewTabRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return newRecordWriter(out, pr, Tab)
}

// newRecordWriter returns a writer instance using sep.
func newRecordWriter(out io.Writer, pr *output.Printing, sep rune) output.RecordWriter {
	cw := csv.NewWriter(out)
	cw.Comma = sep
	return &RecordWriter{needsHeader: pr.ShowHeader, cw: cw, pr: pr}
}

// SetComma sets the CSV writer comma value.
func (w *RecordWriter) SetComma(c rune) {
	w.cw.Comma = c
}

// Open implements output.RecordWriter.
func (w *RecordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	return nil
}

// Flush implements output.RecordWriter.
func (w *RecordWriter) Flush() error {
	w.cw.Flush()
	return nil
}

// Close implements output.RecordWriter.
func (w *RecordWriter) Close() error {
	w.cw.Flush()
	return w.cw.Error()
}

// WriteRecords implements output.RecordWriter.
func (w *RecordWriter) WriteRecords(recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.needsHeader {
		headerRow := w.recMeta.MungedNames()
		for i := range headerRow {
			headerRow[i] = w.pr.Header.Sprint(headerRow[i])
		}
		err := w.cw.Write(headerRow)
		if err != nil {
			return errz.Wrap(err, "failed to write header record")
		}
		w.needsHeader = false
	}

	for _, rec := range recs {
		fields := make([]string, len(rec))

		for i, val := range rec {
			switch val := val.(type) {
			default:
				// should never happen
				fields[i] = fmt.Sprintf("%v", val)
			case nil:
				// nil is rendered as empty string, which this cell already is
			case int64:
				fields[i] = w.pr.Number.Sprint(strconv.FormatInt(val, 10))
			case string:
				fields[i] = w.pr.String.Sprint(val)
			case bool:
				fields[i] = w.pr.Bool.Sprint(val)
			case float64:
				fields[i] = w.pr.Number.Sprintf("%v", val)
			case []byte:
				var size int
				if val != nil {
					size = len(val)
				}
				fields[i] = w.pr.Bytes.Sprintf("[%d bytes]", size)
			case time.Time:
				switch w.recMeta[i].Kind() { //nolint:exhaustive
				default:
					fields[i] = w.pr.Datetime.Sprint(w.pr.FormatDatetime(val))
				case kind.Time:
					fields[i] = w.pr.Datetime.Sprint(w.pr.FormatTime(val))
				case kind.Date:
					fields[i] = w.pr.Datetime.Sprint(w.pr.FormatDate(val))
				}
			}
		}

		err := w.cw.Write(fields)
		if err != nil {
			return errz.Wrap(err, "failed to write records")
		}
	}

	w.cw.Flush()
	return w.cw.Error()
}
