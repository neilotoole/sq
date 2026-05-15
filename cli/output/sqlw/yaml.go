package sqlw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewYAMLWriter returns an output.SQLWriter that emits an SQLPayload as
// YAML. Output is colourised via yamlw to match sq's other YAML output.
func NewYAMLWriter(out io.Writer, pr *output.Printing) *YAMLWriter {
	return &YAMLWriter{out: out, pr: pr}
}

// YAMLWriter is the SQLWriter implementation for the yaml format.
type YAMLWriter struct {
	out io.Writer
	pr  *output.Printing
}

// Render implements output.SQLWriter.
func (w *YAMLWriter) Render(p output.SQLPayload) error {
	s, err := yamlw.MarshalToString(w.pr, p)
	if err != nil {
		return errz.Err(err)
	}
	_, err = io.WriteString(w.out, s)
	return errz.Err(err)
}
