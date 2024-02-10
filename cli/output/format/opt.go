package format

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
)

var _ options.Opt = Opt{}

// NewOpt returns a new format.Opt instance. If validFn is non-nil, it
// is executed against possible values.
func NewOpt(key string, flag *options.Flag, defaultVal Format,
	validFn func(Format) error, usage, help string,
) Opt {
	opt := options.NewBaseOpt(key, flag, usage, help, options.TagOutput)
	return Opt{BaseOpt: opt, defaultVal: defaultVal, validFn: validFn}
}

// Opt is an options.Opt for format.Format.
type Opt struct {
	validFn    func(Format) error
	defaultVal Format
	options.BaseOpt
}

// Process implements options.Processor. It converts matching
// string values in o into format.Format. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op Opt) Process(o options.Options) (options.Options, error) {
	if o == nil {
		return nil, nil
	}

	key := op.Key()
	v, ok := o[key]
	if !ok || v == nil {
		return o, nil
	}

	// v should be a string
	switch v := v.(type) {
	case string:
		// continue below
	case Format:
		return o, nil
	default:
		return nil, errz.Errorf("option {%s} should be {%T} or {%T} but got {%T}: %v",
			key, Format(""), "", v, v)
	}

	var s string
	s, ok = v.(string)
	if !ok {
		return nil, errz.Errorf("option {%s} should be {%T} but got {%T}: %v",
			key, s, v, v)
	}

	var f Format
	if err := f.UnmarshalText([]byte(s)); err != nil {
		return nil, errz.Wrapf(err, "option {%s} is not a valid {%T}", key, f)
	}

	if op.validFn != nil {
		if err := op.validFn(f); err != nil {
			return nil, err
		}
	}

	o = o.Clone()
	o[key] = f
	return o, nil
}

// GetAny implements options.Opt.
func (op Opt) GetAny(o options.Options) any {
	return op.Get(o)
}

// Default returns the default value of op.
func (op Opt) Default() Format {
	return op.defaultVal
}

// DefaultAny implements options.Opt.
func (op Opt) DefaultAny() any {
	return op.defaultVal
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op Opt) Get(o options.Options) Format {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.Key()]
	if !ok {
		return op.defaultVal
	}

	var f Format
	f, ok = v.(Format)
	if !ok {
		return op.defaultVal
	}

	return f
}
