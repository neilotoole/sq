package raww

import (
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/cli/output"
)

// recordWriter implements output.RecordWriter for raw output.
// This is typically used to output a single blob result, such
// as a gif etc. The elements of each record are directly
// written to the backing writer without any separator, or
// encoding, etc.
type recordWriter struct {
	mu      sync.Mutex
	out     io.Writer
	pr      *output.Printing
	recMeta record.Meta
}

// NewRecordWriter returns an output.RecordWriter instance for
// raw output. This is typically used to output a single blob result,
// such as a gif etc. The elements of each record are directly
// written to the backing writer without any separator, or
// encoding, etc..
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, pr: pr}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

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
				switch w.recMeta[i].Kind() { //nolint:exhaustive
				default:
					fmt.Fprint(w.out, w.pr.FormatDatetime(*val))
				case kind.Time:
					fmt.Fprint(w.out, w.pr.FormatTime(*val))
				case kind.Date:
					fmt.Fprint(w.out, w.pr.FormatDate(*val))
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
