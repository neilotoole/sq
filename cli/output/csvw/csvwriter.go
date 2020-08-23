// Package csvw implements writers for CSV.
package csvw

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	// Tab is the tab rune.
	Tab = '\t'

	// Comma is the comma rune.
	Comma = ','
)

// RecordWriter implements output.RecordWriter.
type RecordWriter struct {
	recMeta     sqlz.RecordMeta
	csvW        *csv.Writer
	needsHeader bool
}

// NewRecordWriter returns a writer instance.
func NewRecordWriter(out io.Writer, header bool, sep rune) *RecordWriter {
	csvW := csv.NewWriter(out)
	csvW.Comma = sep
	return &RecordWriter{needsHeader: header, csvW: csvW}
}

// Open implements output.RecordWriter.
func (w *RecordWriter) Open(recMeta sqlz.RecordMeta) error {
	w.recMeta = recMeta
	return nil
}

// Flush implements output.RecordWriter.
func (w *RecordWriter) Flush() error {
	w.csvW.Flush()
	return nil
}

// Close implements output.RecordWriter.
func (w *RecordWriter) Close() error {
	w.csvW.Flush()
	return w.csvW.Error()
}

// WriteRecords implements output.RecordWriter.
func (w *RecordWriter) WriteRecords(recs []sqlz.Record) error {
	if w.needsHeader {
		headerRow := w.recMeta.Names()

		err := w.csvW.Write(headerRow)
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
			case *int64:
				fields[i] = strconv.FormatInt(*val, 10)
			case *string:
				fields[i] = *val
			case *bool:
				fields[i] = strconv.FormatBool(*val)
			case *float64:
				fields[i] = fmt.Sprintf("%v", *val)
			case *[]byte:
				fields[i] = fmt.Sprintf("%v", string(*val))
			case *time.Time:
				switch w.recMeta[i].Kind() {
				default:
					fields[i] = val.Format(stringz.DatetimeFormat)
				case sqlz.KindTime:
					fields[i] = val.Format(stringz.TimeFormat)
				case sqlz.KindDate:
					fields[i] = val.Format(stringz.DateFormat)
				}
			}
		}

		err := w.csvW.Write(fields)
		if err != nil {
			return errz.Wrap(err, "failed to write records")
		}
	}

	w.csvW.Flush()
	return w.csvW.Error()
}
