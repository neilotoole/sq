package yamlw

import (
	"io"

	"github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/commonw"
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
func (w *configWriter) Location(loc string, origin config.Origin) error {
	type cfgInfo struct {
		Location string        `json:"location"`
		Origin   config.Origin `json:"origin,omitempty"`
	}

	c := cfgInfo{
		Location: loc,
		Origin:   origin,
	}

	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), c)
}

// CacheLocation implements output.ConfigWriter.
func (w *configWriter) CacheLocation(loc string) error {
	m := map[string]string{"location": loc}
	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), m)
}

// CacheStat implements output.ConfigWriter.
func (w *configWriter) CacheStat(loc string, enabled bool, size int64) error {
	type cacheInfo struct { //nolint:govet // field alignment
		Location string `yaml:"location"`
		Enabled  bool   `yaml:"enabled"`
		Size     *int64 `yaml:"size,omitempty"`
	}

	ci := cacheInfo{Location: loc, Enabled: enabled}
	if size != -1 {
		ci.Size = &size
	}

	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), ci)
}

// Opt implements output.ConfigWriter.
func (w *configWriter) Opt(o options.Options, opt options.Opt) error {
	if o == nil || opt == nil {
		return nil
	}

	o2 := options.Options{opt.Key(): o[opt.Key()]}

	if !w.pr.Verbose {
		return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), o2)
	}

	vo := commonw.NewVerboseOpt(opt, o2)
	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), vo)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(reg *options.Registry, o options.Options) error {
	if len(o) == 0 && !w.pr.Verbose {
		return nil
	}

	if !w.pr.Verbose {
		return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), o)
	}

	opts := reg.Opts()
	m := map[string]commonw.VerboseOpt{}
	for _, opt := range opts {
		m[opt.Key()] = commonw.NewVerboseOpt(opt, o)
	}

	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), m)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(o options.Options, opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	vo := commonw.NewVerboseOpt(opt, o)
	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), vo)
}

// UnsetOption implements output.ConfigWriter.
func (w *configWriter) UnsetOption(opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	o := options.Options{opt.Key(): opt.GetAny(nil)}
	vo := commonw.NewVerboseOpt(opt, o)
	return writeYAML(w.out, w.p, newDecimalMarshaler(w.pr.DecimalAsNumber), vo)
}
