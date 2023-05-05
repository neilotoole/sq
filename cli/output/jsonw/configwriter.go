package jsonw

import (
	"io"
	"reflect"

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

// Opt implements output.ConfigWriter.
func (w *configWriter) Opt(reg *options.Registry, o options.Options, opt options.Opt) error {
	if reg == nil || o == nil || opt == nil {
		return nil
	}

	o2 := options.Options{opt.Key(): o[opt.Key()]}

	if !w.pr.Verbose {
		return writeJSON(w.out, w.pr, o2)
	}

	vo := newVerboseOpt(opt, o2)
	return writeJSON(w.out, w.pr, vo)
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(reg *options.Registry, o options.Options) error {
	if len(o) == 0 {
		return nil
	}

	if !w.pr.Verbose {
		return writeJSON(w.out, w.pr, o)
	}

	o2 := o.Clone()
	for _, opt := range reg.Opts() {
		if !o2.IsSet(opt) {
			o2[opt.Key()] = opt.GetAny(nil)
		}
	}

	m := map[string]verboseOpt{}
	for _, key := range o.Keys() {
		m[key] = newVerboseOpt(reg.Get(key), o)
	}

	return writeJSON(w.out, w.pr, m)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(_ *options.Registry, o options.Options, opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	vo := newVerboseOpt(opt, o)
	return writeJSON(w.out, w.pr, vo)
}

// UnsetOption implements output.ConfigWriter.
func (w *configWriter) UnsetOption(opt options.Opt) error {
	if !w.pr.Verbose {
		return nil
	}

	o := options.Options{opt.Key(): opt.GetAny(nil)}
	vo := newVerboseOpt(opt, o)
	return writeJSON(w.out, w.pr, vo)
}

// verboseOpt is a verbose realization of an options.Opt value.
type verboseOpt struct {
	Key          string `json:"key"`
	Type         string `json:"type"`
	IsSet        bool   `json:"is_set"`
	DefaultValue any    `json:"default_value"`
	Value        any    `json:"value"`
	Comment      string `json:"comment"`
}

func newVerboseOpt(opt options.Opt, o options.Options) verboseOpt {
	v := verboseOpt{
		Key:          opt.Key(),
		DefaultValue: opt.GetAny(nil),
		IsSet:        o.IsSet(opt),
		Comment:      opt.Comment(),
		Value:        opt.GetAny(o),
		Type:         reflect.TypeOf(opt.GetAny(nil)).String(),
	}

	return v
}
