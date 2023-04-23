package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/config/options"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, pr *output.Printing) output.ConfigWriter {
	return &configWriter{out: out, pr: pr}
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

	return writeJSON(w.out, w.pr, c)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(opts options.Options) error {
	if opts == nil {
		return nil
	}

	return writeJSON(w.out, w.pr, opts)
}
