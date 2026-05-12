package sqlw

import (
	"encoding/json"
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewJSONWriter returns an output.SQLWriter that emits an SQLPayload as
// pretty-printed JSON. The indent string is taken from pr.Indent
// (typically two spaces). The Compact field on pr is honoured: if
// true, output is compact.
func NewJSONWriter(out io.Writer, pr *output.Printing) *JSONWriter {
	return &JSONWriter{out: out, pr: pr, compact: pr.Compact}
}

// NewJSONLWriter returns an output.SQLWriter that emits an SQLPayload as
// compact one-line JSON, suitable for jsonl pipelines.
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
	var (
		b   []byte
		err error
	)
	if w.compact {
		b, err = json.Marshal(p)
	} else {
		indent := w.pr.Indent
		if indent == "" {
			indent = "  "
		}
		b, err = json.MarshalIndent(p, "", indent)
	}
	if err != nil {
		return errz.Err(err)
	}

	if _, err = w.out.Write(b); err != nil {
		return errz.Err(err)
	}
	if _, err = w.out.Write([]byte("\n")); err != nil {
		return errz.Err(err)
	}
	return nil
}
