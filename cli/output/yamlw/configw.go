package yamlw

import (
	"io"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	p   printer.Printer
	out io.Writer
	pr  *output.Printing
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, pr *output.Printing) output.ConfigWriter {
	return &configWriter{out: out, pr: pr, p: newPrinter(pr)}
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

	return writeYAML(w.p, w.out, c)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(opts options.Options) error {
	if opts == nil {
		return nil
	}

	return writeYAML(w.p, w.out, opts)
}
