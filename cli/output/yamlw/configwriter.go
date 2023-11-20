package yamlw

import (
	"io"

	"github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/outputx"
	"github.com/neilotoole/sq/libsq/core/options"
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

	return writeYAML(w.out, w.p, c)
}

// Opt implements output.ConfigWriter.
func (w *configWriter) Opt(o options.Options, opt options.Opt) error {
	if o == nil || opt == nil {
		return nil
	}

	o2 := options.Options{opt.Key(): o[opt.Key()]}

	if !w.pr.Verbose {
		return writeYAML(w.out, w.p, o2)
	}

	vo := outputx.NewVerboseOpt(opt, o2)
	return writeYAML(w.out, w.p, vo)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(reg *options.Registry, o options.Options) error {
	if len(o) == 0 && !w.pr.Verbose {
		return nil
	}

	if !w.pr.Verbose {
		return writeYAML(w.out, w.p, o)
	}

	opts := reg.Opts()
	m := map[string]outputx.VerboseOpt{}
	for _, opt := range opts {
		m[opt.Key()] = outputx.NewVerboseOpt(opt, o)
	}

	return writeYAML(w.out, w.p, m)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(o options.Options, opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	vo := outputx.NewVerboseOpt(opt, o)
	return writeYAML(w.out, w.p, vo)
}

// UnsetOption implements output.ConfigWriter.
func (w *configWriter) UnsetOption(opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	o := options.Options{opt.Key(): opt.GetAny(nil)}
	vo := outputx.NewVerboseOpt(opt, o)
	return writeYAML(w.out, w.p, vo)
}
