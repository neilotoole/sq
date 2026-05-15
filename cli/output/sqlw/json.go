package sqlw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
)

// NewJSONWriter returns an output.SQLWriter that emits an SQLPayload
// as JSON, pretty-printed by default but compact when pr.Compact is
// true. For unconditionally single-line output (e.g. for jsonl
// pipelines), use [NewJSONLWriter] instead. Output honors the color
// palette in pr when color is enabled, matching the rest of sq's
// JSON output.
func NewJSONWriter(out io.Writer, pr *output.Printing) *JSONWriter {
	return &JSONWriter{out: out, pr: pr, compact: pr.Compact}
}

// NewJSONLWriter returns an output.SQLWriter that emits an SQLPayload as
// compact one-line JSON, suitable for jsonl pipelines. Color is honored
// when pr has color enabled.
func NewJSONLWriter(out io.Writer, pr *output.Printing) *JSONWriter {
	return &JSONWriter{out: out, pr: pr, compact: true}
}

// JSONWriter is the SQLWriter implementation for the json and jsonl
// formats. compact toggles between pretty-printed and single-line
// output.
type JSONWriter struct {
	out     io.Writer
	pr      *output.Printing
	compact bool
}

// Render implements output.SQLWriter.
func (w *JSONWriter) Render(p output.SQLPayload) error {
	pr := w.pr
	if pr.Compact != w.compact {
		pr = pr.Clone()
		pr.Compact = w.compact
	}
	return jsonw.WriteJSON(w.out, pr, p)
}
