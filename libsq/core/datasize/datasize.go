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

// Parse parses t as a ByteSize, e.g. "10MB". On error, the returned
// error is wrapped via errz so it carries a stack trace.
func Parse(t []byte) (ByteSize, error) {
	v, err := datasize.Parse(t)
	if err != nil {
		return v, errz.Err(err)
	}
	return v, nil
}

// MustParse is like Parse, but panics on error. It is intended for use
// with constant input, typically at package-init.
func MustParse(t []byte) ByteSize {
	v, err := Parse(t)
	if err != nil {
		panic(err)
	}
	return v
}

// ParseString is like Parse, but accepts a string.
func ParseString(s string) (ByteSize, error) {
	return Parse([]byte(s))
}

// MustParseString is like MustParse, but accepts a string.
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

	if _, isByteSize := v.(ByteSize); isByteSize {
		return o, nil
	}

	f, err := op.convert(v)
	if err != nil {
		return nil, err
	}

	o = o.Clone()
	o[key] = f
	return o, nil
}

// convert coerces v into a ByteSize. A ByteSize reaches the config only as a
// string or a number, so Get needs this to avoid silently returning op's
// default for an Options that has not been through Registry.Process.
// See #1209.
func (op Opt) convert(v any) (ByteSize, error) {
	key := op.Key()
	switch v := v.(type) {
	case ByteSize:
		return v, nil
	case uint:
		return ByteSize(v), nil
	case uint64:
		return ByteSize(v), nil
	case int:
		return ByteSize(v), nil //nolint:gosec // ignore overflow concern
	case int64:
		return ByteSize(v), nil //nolint:gosec // ignore overflow concern
	case string:
		var f ByteSize
		if err := f.UnmarshalText([]byte(v)); err != nil {
			return 0, errz.Wrapf(err, "option {%s} is not a valid {%T}", key, f)
		}
		return f, nil
	default:
		return 0, errz.Errorf("option {%s} should be a string, an integer, or {%T} but got {%T}: %v",
			key, ByteSize(0), v, v)
	}
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
	if !ok || v == nil {
		return op.defaultVal
	}

	f, err := op.convert(v)
	if err != nil {
		return op.defaultVal
	}

	return f
}
