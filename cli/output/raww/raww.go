package raww

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// recordWriter implements output.RecordWriter for raw output.
// This is typically used to output a single blob result, such
// as a gif etc. The elements of each record are directly
// written to the backing writer without any separator, or
// encoding, etc.
type recordWriter struct {
	out     io.Writer
	recMeta sqlz.RecordMeta
}

// NewRecordWriter returns an output.RecordWriter instance for
// raw output. This is typically used to output a single blob result,
// such as a gif etc. The elements of each record are directly
// written to the backing writer without any separator, or
// encoding, etc..
func NewRecordWriter(out io.Writer) output.RecordWriter {
	return &recordWriter{out: out}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta sqlz.RecordMeta) error {
	w.recMeta = recMeta
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []sqlz.Record) error {
	if len(recs) == 0 {
		return nil
	}

	for _, rec := range recs {
		for i, val := range rec {
			switch val := val.(type) {
			case nil:
			case *[]byte:
				_, _ = w.out.Write(*val)
			case *string:
				_, _ = w.out.Write([]byte(*val))
			case *bool:
				fmt.Fprint(w.out, strconv.FormatBool(*val))
			case *int64:
				fmt.Fprint(w.out, strconv.FormatInt(*val, 10))
			case *float64:
				fmt.Fprint(w.out, stringz.FormatFloat(*val))
			case *time.Time:
				switch w.recMeta[i].Kind() {
				default:
					fmt.Fprint(w.out, val.Format(stringz.DatetimeFormat))
				case sqlz.KindTime:
					fmt.Fprint(w.out, val.Format(stringz.TimeFormat))
				case sqlz.KindDate:
					fmt.Fprint(w.out, val.Format(stringz.DateFormat))
				}
			default:
				// should never happen
				fmt.Fprintf(w.out, "%s", val)
			}
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
	return nil
}
