// Package htmlw implements a RecordWriter for HTML.
package htmlw

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// RecordWriter implements output.RecordWriter.
type recordWriter struct {
	mu      sync.Mutex
	recMeta record.Meta
	pr      *output.Printing
	out     io.Writer
	buf     *bytes.Buffer
}

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter an output.RecordWriter for HTML.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &recordWriter{out: out, pr: pr}
}

// Open implements output.RecordWriter.
func (w *recordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	w.buf = &bytes.Buffer{}

	const header = `<!doctype html>
<html>
<head>
<title>sq output</title>
</head>
<body>

<table>
  <colgroup>
`

	w.buf.WriteString(header)
	for _, field := range recMeta {
		w.buf.WriteString("    <col class=\"kind-")
		w.buf.WriteString(field.Kind().String())
		w.buf.WriteString("\" />\n")
	}
	w.buf.WriteString("  </colgroup>\n  <thead>\n    <tr>\n")
	for _, field := range recMeta {
		w.buf.WriteString("      <th scope=\"col\">")
		w.buf.WriteString(field.Name())
		w.buf.WriteString("</th>\n")
	}
	w.buf.WriteString("    </tr>\n  </thead>\n  <tbody>\n")
	return nil
}

// Flush implements output.RecordWriter.
func (w *recordWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.buf.WriteTo(w.out) // resets buf
	return errz.Err(err)
}

// Close implements output.RecordWriter.
func (w *recordWriter) Close() error {
	err := w.Flush()
	if err != nil {
		return err
	}

	const footer = `  </tbody>
</table>

</body>
</html>
`

	_, err = w.out.Write([]byte(footer))
	return errz.Err(err)
}

func (w *recordWriter) writeRecord(rec record.Record) error {
	w.buf.WriteString("    <tr>\n")

	var s string
	for i, field := range rec {
		w.buf.WriteString("      <td>")

		switch val := field.(type) {
		default:
			s = html.EscapeString(fmt.Sprintf("%v", val))
			// should never happen
		case nil:
			// nil is rendered as empty string, which this cell already is
		case *int64:
			s = strconv.FormatInt(*val, 10)
		case *string:
			s = html.EscapeString(*val)
		case *bool:
			s = strconv.FormatBool(*val)
		case *float64:
			s = stringz.FormatFloat(*val)
		case *[]byte:
			s = base64.StdEncoding.EncodeToString(*val)
		case *time.Time:
			switch w.recMeta[i].Kind() { //nolint:exhaustive
			default:
				s = w.pr.FormatDatetime(*val)
			case kind.Time:
				s = w.pr.FormatTime(*val)
			case kind.Date:
				s = w.pr.FormatDate(*val)
			}
		}

		w.buf.WriteString(s)
		w.buf.WriteString("</td>\n")
	}

	w.buf.WriteString("    </tr>\n")

	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *recordWriter) WriteRecords(recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var err error
	for _, rec := range recs {
		err = w.writeRecord(rec)
		if err != nil {
			return err
		}
	}

	return nil
}
