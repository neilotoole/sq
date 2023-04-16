package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, fm *output.Formatting) output.ConfigWriter {
	return &configWriter{out: out, fm: fm}
}

// Dir implements output.ConfigWriter.
func (w *configWriter) Dir(path, origin string) error {
	type dirInfo struct {
		Path   string `json:"path"`
		Origin string `json:"origin,omitempty"`
	}

	d := dirInfo{
		Path:   path,
		Origin: origin,
	}

	return writeJSON(w.out, w.fm, d)
}
