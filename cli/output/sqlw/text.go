// Package sqlw provides output.SQLWriter implementations for the
// --dry-run mode of the slq command, where the rendered SQL is printed
// instead of being executed.
package sqlw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
)

// NewTextWriter returns an output.SQLWriter that emits plain SQL
// followed by a newline. When pr has color enabled, the SQL is also
// syntax-highlighted to blend with the rest of sq's terminal output;
// when pr is monochrome the SQL is written as-is.
func NewTextWriter(out io.Writer, pr *output.Printing) *TextWriter {
	return &TextWriter{out: out, pr: pr}
}

// TextWriter is the SQLWriter implementation for the text/raw formats.
type TextWriter struct {
	out io.Writer
	pr  *output.Printing
}

// Render implements output.SQLWriter.
func (w *TextWriter) Render(p output.SQLPayload) error {
	sql := p.SQL
	if !w.pr.IsMonochrome() {
		sql = highlight(sql, w.pr)
	}
	_, err := io.WriteString(w.out, sql+"\n")
	return err
}

// highlight is a no-op placeholder; Task 6 replaces this with the
// chroma-based implementation.
func highlight(sql string, _ *output.Printing) string {
	return sql
}
