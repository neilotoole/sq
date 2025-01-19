// Package datasize provides a ByteSize type for representing data sizes in bytes,
// and functions for parsing and formatting ByteSize values.
package datasize

import (
	"github.com/c2h5oh/datasize"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
)

// ByteSize is a type for representing data sizes in bytes.
type ByteSize = datasize.ByteSize

func Parse(t []byte) (ByteSize, error) {
	return datasize.Parse(t)
}

func MustParse(t []byte) ByteSize {
	return datasize.MustParse(t)
}

func ParseString(s string) (ByteSize, error) {
	return Parse([]byte(s))
}

func MustParseString(s string) ByteSize {
	return MustParse([]byte(s))
}

var _ options.Opt = Opt{}

// Opt is an options.Opt for format.Format.
type Opt struct {
	options.BaseOpt
	defaultVal ByteSize
}

// NewOpt returns a new datasize.Opt instance.
func NewOpt(key string, flag *options.Flag, defaultVal ByteSize,
	usage, help string, tags ...string,
) Opt {
	opt := options.NewBaseOpt(key, flag, usage, help, tags...)
	return Opt{BaseOpt: opt, defaultVal: defaultVal}
}

// Process implements options.Processor. It converts matching
// string or integer values in o into ByteSize. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op Opt) Process(o options.Options) (options.Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	key := op.Key()
	v, ok := o[key]
	if !ok || v == nil {
		return o, nil
	}

	switch v := v.(type) {
	case string:
		// continue below
	case ByteSize:
		return o, nil
	case uint:
		o = o.Clone()
		o[key] = ByteSize(v)
		return o, nil
	case uint64:
		o = o.Clone()
		o[key] = ByteSize(v)
		return o, nil
	case int:
		o = o.Clone()
		o[key] = ByteSize(v) //nolint:gosec // ignore overflow concern
		return o, nil
	case int64:
		o = o.Clone()
		o[key] = ByteSize(v) //nolint:gosec // ignore overflow concern
		return o, nil
	default:
		return nil, errz.Errorf("option {%s} should be a string, int, or {%T} but got {%T}: %v",
			ByteSize(0), "", v, v)
	}

	var s string
	s, ok = v.(string)
	if !ok {
		return nil, errz.Errorf("option {%s} should be {%T} but got {%T}: %v",
			key, s, v, v)
	}

	var f ByteSize
	if err := f.UnmarshalText([]byte(s)); err != nil {
		return nil, errz.Wrapf(err, "option {%s} is not a valid {%T}", key, f)
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
func (op Opt) Default() ByteSize {
	return op.defaultVal
}

// DefaultAny implements options.Opt.
func (op Opt) DefaultAny() any {
	return op.defaultVal
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op Opt) Get(o options.Options) ByteSize {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.Key()]
	if !ok {
		return op.defaultVal
	}

	var f ByteSize
	f, ok = v.(ByteSize)
	if !ok {
		return op.defaultVal
	}

	return f
}
