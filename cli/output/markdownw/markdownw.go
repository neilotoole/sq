// Package markdownw implements writers for Markdown.
package markdownw

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// RecordWriter implements output.RecordWriter.
type RecordWriter struct {
	mu      sync.Mutex
	recMeta record.Meta
	pr      *output.Printing
	out     io.Writer
	buf     *bytes.Buffer
}

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns a writer instance.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &RecordWriter{out: out, pr: pr}
}

// Open implements output.RecordWriter.
func (w *RecordWriter) Open(recMeta record.Meta) error {
	w.recMeta = recMeta
	w.buf = &bytes.Buffer{}

	// Write the header
	for i, field := range recMeta {
		w.buf.WriteString("| ")
		w.buf.WriteString(field.MungedName() + " ")

		if i == len(recMeta)-1 {
			w.buf.WriteString("|\n")
		}
	}

	// Write the header separator row
	for i := range recMeta {
		w.buf.WriteString("| --- ")

		if i == len(recMeta)-1 {
			w.buf.WriteString("|\n")
		}
	}

	return nil
}

// Flush implements output.RecordWriter.
func (w *RecordWriter) Flush() error {
	_, err := w.buf.WriteTo(w.out) // resets buf
	return err
}

// Close implements output.RecordWriter.
func (w *RecordWriter) Close() error {
	return w.Flush()
}

func (w *RecordWriter) writeRecord(rec record.Record) error {
	var s string
	for i, field := range rec {
		w.buf.WriteString("| ")

		switch val := field.(type) {
		default:
			// should never happen
			s = escapeMarkdown(fmt.Sprintf("%v", val))

		case nil:
			// nil is rendered as empty string, which this cell already is
		case int64:
			s = strconv.FormatInt(val, 10)
		case string:
			s = escapeMarkdown(val)
		case bool:
			s = strconv.FormatBool(val)
		case float64:
			s = stringz.FormatFloat(val)
		case []byte:
			s = base64.StdEncoding.EncodeToString(val)
		case time.Time:
			switch w.recMeta[i].Kind() { //nolint:exhaustive
			default:
				s = w.pr.FormatDatetime(val)
			case kind.Time:
				s = w.pr.FormatTime(val)
			case kind.Date:
				s = w.pr.FormatDate(val)
			}
		}

		w.buf.WriteString(s + " ")
	}

	w.buf.WriteString("|\n")
	return nil
}

// WriteRecords implements output.RecordWriter.
func (w *RecordWriter) WriteRecords(recs []record.Record) error {
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

// escapeMarkdown is quick effort at escaping markdown
// table cell text. It is not at all tested. Replace this
// function with a real library call at the earliest opportunity.
func escapeMarkdown(s string) string {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "|", "&vert;")
	s = strings.ReplaceAll(s, "\r\n", "<br/>")
	s = strings.ReplaceAll(s, "\n", "<br/>")
	return s
}
