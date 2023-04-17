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

// Location implements output.ConfigWriter.
func (w *configWriter) Location(loc, origin string) error {
	type cfgInfo struct {
		Location string `json:"location"`
		Origin   string `json:"origin,omitempty"`
	}

	c := cfgInfo{
		Location: loc,
		Origin:   origin,
	}

	return writeJSON(w.out, w.fm, c)
}
