// Package markdownw implements writers for Markdown.
package markdownw

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// RecordWriter implements output.RecordWriter.
type RecordWriter struct {
	out     io.Writer
	pr      *output.Printing
	buf     *bytes.Buffer
	recMeta record.Meta
	mu      sync.Mutex
}

var _ output.NewRecordWriterFunc = NewRecordWriter

// NewRecordWriter returns a writer instance.
func NewRecordWriter(out io.Writer, pr *output.Printing) output.RecordWriter {
	return &RecordWriter{out: out, pr: pr}
}

// Open implements output.RecordWriter.
func (w *RecordWriter) Open(_ context.Context, recMeta record.Meta) error {
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
func (w *RecordWriter) Flush(context.Context) error {
	_, err := w.buf.WriteTo(w.out) // resets buf
	return err
}

// Close implements output.RecordWriter.
func (w *RecordWriter) Close(ctx context.Context) error {
	return w.Flush(ctx)
}

func (w *RecordWriter) writeRecord(rec record.Record) error { //nolint:unparam
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
		case decimal.Decimal:
			s = stringz.FormatDecimal(val)
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
func (w *RecordWriter) WriteRecords(ctx context.Context, recs []record.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error
	for _, rec := range recs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
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

// writeTablesTOC writes a compact one-line table of contents: a middot-
// separated list of links to each table/view section (in the given order).
// The link targets use mdAnchor so they match the heading anchors that
// Markdown renderers (e.g. GitHub) auto-generate from the `### `+"`name`"
// headings.
func writeTablesTOC(buf *bytes.Buffer, tables []*metadata.Table) {
	links := make([]string, len(tables))
	for i, tbl := range tables {
		link := fmt.Sprintf("[%s](#%s)", mdCode(tbl.Name), mdAnchor(tbl.Name))
		if commonw.IsView(tbl) {
			// Markdown can't tint the link like HTML; italicize views instead.
			link = "*" + link + "*"
		}
		links[i] = link
	}
	buf.WriteString(strings.Join(links, " · ") + "\n")
}

// mdAnchor returns the heading anchor a Markdown renderer generates for a
// table heading (`### `+"`name`"): lower-cased, with characters outside
// [a-z0-9_-] dropped and spaces turned into hyphens. For ordinary snake_case
// identifiers this is just the name.
func mdAnchor(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// checkMark returns "✓" when b is true, else "" — for centered boolean
// table cells such as an index's Unique / Primary columns.
func checkMark(b bool) string {
	if b {
		return "✓"
	}
	return ""
}

// mdCode renders s as a Markdown inline-code span (backtick-quoted), used
// for identifiers such as table and column names so they read as code
// rather than prose. An empty string yields an empty span (no backticks).
func mdCode(s string) string {
	if s == "" {
		return ""
	}
	return mdCodeSpan(s)
}

// mdCodeCell renders s as a Markdown inline-code span for use inside a
// table cell, escaping the pipe and newline characters that would
// otherwise break the cell. An empty string yields an empty cell (no
// backticks).
func mdCodeCell(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return mdCodeSpan(s)
}

// mdCodeSpan wraps non-empty s in a backtick-delimited inline-code span. The
// fence is one backtick longer than the longest backtick run in s, so embedded
// backticks can't close the span early; and when s starts or ends with a
// backtick it's padded with a space, which CommonMark strips as a matching
// leading/trailing pair. Ordinary identifiers (no backticks) get a plain
// single-backtick span.
func mdCodeSpan(s string) string {
	longest, cur := 0, 0
	for _, r := range s {
		if r == '`' {
			cur++
			if cur > longest {
				longest = cur
			}
		} else {
			cur = 0
		}
	}
	fence := strings.Repeat("`", longest+1)
	pad := ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		pad = " "
	}
	return fence + pad + s + pad + fence
}
