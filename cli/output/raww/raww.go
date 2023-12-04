package raww

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
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

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns an output.RecordWriter instance for
// raw output. This is typically used to output a single blob result,
// such as a gif etc. The elements of each record are directly
// written to the backing writer without any separator, or
// encoding, etc..
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, pr: pr}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(_ context.Context, recMeta record.Meta) error {
	w.recMeta = recMeta
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(recs) == 0 {
		return nil
	}

	for _, rec := range recs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		for i, val := range rec {
			switch val := val.(type) {
			case nil:
			case []byte:
				_, _ = w.out.Write(val)
			case string:
				_, _ = w.out.Write([]byte(val))
			case bool:
				fmt.Fprint(w.out, strconv.FormatBool(val))
			case int64:
				fmt.Fprint(w.out, strconv.FormatInt(val, 10))
			case float64:
				fmt.Fprint(w.out, stringz.FormatFloat(val))
			case decimal.Decimal:
				fmt.Fprint(w.out, stringz.FormatDecimal(val))
			case time.Time:
				switch w.recMeta[i].Kind() { //nolint:exhaustive
				default:
					fmt.Fprint(w.out, w.pr.FormatDatetime(val))
				case kind.Time:
					fmt.Fprint(w.out, w.pr.FormatTime(val))
				case kind.Date:
					fmt.Fprint(w.out, w.pr.FormatDate(val))
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
func (w *recordWriter) Flush(context.Context) error {
	return nil
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close(context.Context) error {
	return nil
}
