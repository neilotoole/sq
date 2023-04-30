package jsonw

import (
	"io"

	"github.com/neilotoole/sq/libsq/core/options"

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
func (w *configWriter) Options(_ *options.Registry, o options.Options) error {
	if o == nil {
		return nil
	}

	return writeJSON(w.out, w.pr, o)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(_ *options.Registry, o options.Options, opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	o = options.Effective(o, opt)
	return writeJSON(w.out, w.pr, o)
}
