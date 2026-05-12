package sqlw

import (
	"io"

	goccy "github.com/goccy/go-yaml"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewYAMLWriter returns an output.SQLWriter that emits an SQLPayload as
// YAML. Uses goccy/go-yaml to match the encoder used elsewhere in sq.
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
	b, err := goccy.Marshal(p)
	if err != nil {
		return errz.Err(err)
	}
	if _, err = w.out.Write(b); err != nil {
		return errz.Err(err)
	}
	return nil
}
