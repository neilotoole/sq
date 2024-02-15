package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output/commonw"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/options"
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

// CacheLocation implements output.ConfigWriter.
func (w *configWriter) CacheLocation(loc string) error {
	m := map[string]string{"location": loc}
	return writeJSON(w.out, w.pr, m)
}

// CacheStat implements output.ConfigWriter.
func (w *configWriter) CacheStat(loc string, enabled bool, size int64) error {
	type cacheInfo struct { //nolint:govet // field alignment
		Location string `json:"location"`
		Enabled  bool   `json:"enabled"`
		Size     *int64 `json:"size,omitempty"`
	}

	ci := cacheInfo{Location: loc, Enabled: enabled}
	if size != -1 {
		ci.Size = &size
	}

	return writeJSON(w.out, w.pr, ci)
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

// Opt implements output.ConfigWriter.
func (w *configWriter) Opt(o options.Options, opt options.Opt) error {
	if o == nil || opt == nil {
		return nil
	}

	o2 := options.Options{opt.Key(): o[opt.Key()]}

	if !w.pr.Verbose {
		return writeJSON(w.out, w.pr, o2)
	}

	vo := commonw.NewVerboseOpt(opt, o2)
	return writeJSON(w.out, w.pr, vo)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(reg *options.Registry, o options.Options) error {
	if len(o) == 0 && !w.pr.Verbose {
		return nil
	}

	if !w.pr.Verbose {
		return writeJSON(w.out, w.pr, o)
	}

	opts := reg.Opts()
	m := map[string]commonw.VerboseOpt{}
	for _, opt := range opts {
		m[opt.Key()] = commonw.NewVerboseOpt(opt, o)
	}

	return writeJSON(w.out, w.pr, m)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(o options.Options, opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	vo := commonw.NewVerboseOpt(opt, o)
	return writeJSON(w.out, w.pr, vo)
}

// UnsetOption implements output.ConfigWriter.
func (w *configWriter) UnsetOption(opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	o := options.Options{opt.Key(): opt.GetAny(nil)}
	vo := commonw.NewVerboseOpt(opt, o)
	return writeJSON(w.out, w.pr, vo)
}
