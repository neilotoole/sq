package options

import (
	"fmt"
	"strconv"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Opt is an option type. Concrete impls exist for various types,
// such as options.Int or options.Duration. Each concrete type must implement
// a "Get(o Options) T" method that returns the appropriate type T. It
// should also provide a NewT(...) T constructor. The constructor must
// invoke Registry.Add on options.DefaultRegistry.
//
// An impl can (optionally) implement options.Processor if it needs
// to munge the underlying value. For example, options.Duration converts a
// string such as "1m30s" into a time.Duration.
type Opt interface {
	// Key returns the Opt key, such as "ping.timeout".
	Key() string

	// String returns a log/debug friendly representation.
	String() string

	// IsSet returns true if this Opt is set in opts.
	IsSet(opts Options) bool

	// Tags returns any tags on this Opt instance. For example, an Opt might
	// have tags [source, csv].
	Tags() []string
}

type baseOpt struct {
	key     string
	comment string
	tags    []string
}

// Key implements options.Opt.
func (o baseOpt) Key() string {
	return o.key
}

// IsSet implements options.Opt.
func (o baseOpt) IsSet(opts Options) bool {
	if opts == nil {
		return false
	}

	return opts.IsSet(o)
}

// String implements options.Opt.
func (o baseOpt) String() string {
	return o.key
}

// Tags implements options.Opt.
func (o baseOpt) Tags() []string {
	return o.tags
}

var _ Opt = String{}

// NewString returns an options.String instance.
func NewString(key, defaultVal, comment string, tags ...string) String {
	opt := String{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}

	DefaultRegistry.Add(opt)
	return opt
}

// String is an options.Opt for type string.
type String struct {
	baseOpt
	defaultVal string
}

// Get returns o's value in opts. If opts is nil, or no value
// is set, o's default value is returned.
func (o String) Get(opts Options) string {
	if opts == nil {
		return o.defaultVal
	}

	v, ok := opts[o.key]
	if !ok {
		return o.defaultVal
	}

	var s string
	s, ok = v.(string)
	if !ok {
		return o.defaultVal
	}

	return s
}

var _ Opt = Int{}

// NewInt returns an options.Int instance.
func NewInt(key string, defaultVal int, comment string, tags ...string) Int {
	opt := Int{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
	DefaultRegistry.Add(opt)
	return opt
}

// Int is an options.Opt for type int.
type Int struct {
	baseOpt
	defaultVal int
}

// Get returns o's value in opts. If opts is nil, or no value
// is set, o's default value is returned.
func (o Int) Get(opts Options) int {
	if opts == nil {
		return o.defaultVal
	}

	v, ok := opts[o.key]
	if !ok || v == nil {
		return o.defaultVal
	}

	switch i := v.(type) {
	case int:
		return i
	case int64:
		return int(i)
	case uint64:
		return int(i)
	case uint:
		return int(i)
	default:
		return o.defaultVal
	}
}

// Process implements options.Processor. It converts matching
// values in opts into bool. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (o Int) Process(opts Options) (Options, error) {
	if opts == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := opts[o.key]
	if !ok || v == nil {
		return opts, nil
	}

	if _, ok = v.(int); ok {
		// Happy path
		return opts, nil
	}

	opts = opts.Clone()

	var i int
	switch v := v.(type) {
	case float32:
		i = int(v)
	case float64:
		i = int(v)
	case uint:
		i = int(v)
	case uint8:
		i = int(v)
	case uint16:
		i = int(v)
	case uint32:
		i = int(v)
	case uint64:
		i = int(v)
	case int8:
		i = int(v)
	case int16:
		i = int(v)
	case int32:
		i = int(v)
	case int64:
		i = int(v)
	case string:
		if v == "" {
			// Empty string is effectively nil
			delete(opts, o.key)
			return opts, nil
		}

		var err error
		if i, err = strconv.Atoi(v); err != nil {
			return nil, errz.Wrapf(err, "invalid int value for {%s}: %v", o.key, v)
		}
	default:
		// This shouldn't happen, but it's a last-ditch effort.
		// Print v as a string, and try to parse it.
		s := fmt.Sprintf("%v", v)
		var err error
		if i, err = strconv.Atoi(s); err != nil {
			return nil, errz.Wrapf(err, "invalid int value for {%s}: %v", o.key, v)
		}
	}

	opts[o.key] = i
	return opts, nil
}

var _ Opt = Bool{}

// NewBool returns an options.Bool instance.
func NewBool(key string, defaultVal bool, comment string, tags ...string) Bool {
	opt := Bool{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
	DefaultRegistry.Add(opt)
	return opt
}

// Bool is an options.Opt for type bool.
type Bool struct {
	baseOpt
	defaultVal bool
}

// Get returns o's value in opts. If opts is nil, or no value
// is set, o's default value is returned.
func (o Bool) Get(opts Options) bool {
	if opts == nil {
		return o.defaultVal
	}

	v, ok := opts[o.key]
	if !ok || v == nil {
		return o.defaultVal
	}

	var b bool
	b, ok = v.(bool)
	if !ok {
		return o.defaultVal
	}

	return b
}

// Process implements options.Processor. It converts matching
// string values in opts into bool. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (o Bool) Process(opts Options) (Options, error) {
	if opts == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := opts[o.key]
	if !ok || v == nil {
		return opts, nil
	}

	if _, ok = v.(bool); ok {
		// Happy path
		return opts, nil
	}

	opts = opts.Clone()

	switch v := v.(type) {
	case string:
		if v == "" {
			// Empty string is effectively nil
			delete(opts, o.key)
			return opts, nil
		}

		// It could be a string like "true"
		b, err := stringz.ParseBool(v)
		if err != nil {
			return nil, errz.Wrapf(err, "invalid bool value for {%s}: %v", o.key, v)
		}
		opts[o.key] = b
	default:
		// Well, we don't know what this is... maybe a number like "1"?
		// Last-ditch effort. Print the value to a string, and check
		// if we can parse the string into a bool.
		s := fmt.Sprintf("%v", v)
		b, err := stringz.ParseBool(s)
		if err != nil {
			return nil, errz.Wrapf(err, "invalid bool value for {%s}: %v", o.key, v)
		}
		opts[o.key] = b
	}

	return opts, nil
}

var _ Opt = Duration{}

// NewDuration returns an options.Duration instance.
func NewDuration(key string, defaultVal time.Duration, comment string, tags ...string) Duration {
	opt := Duration{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
	DefaultRegistry.Add(opt)
	return opt
}

// Duration is an options.Opt for time.Duration.
type Duration struct {
	baseOpt
	defaultVal time.Duration
}

// Process implements options.Processor. It converts matching
// string values in opts into time.Duration. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (o Duration) Process(opts Options) (Options, error) {
	if opts == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := opts[o.key]
	if !ok || v == nil {
		return opts, nil
	}

	// v should be a string
	var s string
	s, ok = v.(string)
	if !ok {
		return nil, errz.Errorf("option {%s} should be {%T} but got {%T}: %v",
			o.key, s, v, v)
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, errz.Wrapf(err, "options {%s} is not a valid duration", o.key)
	}

	opts = opts.Clone()
	opts[o.key] = d
	return opts, nil
}

// Get returns o's value in opts. If opts is nil, or no value
// is set, o's default value is returned.
func (o Duration) Get(opts Options) time.Duration {
	if opts == nil {
		return o.defaultVal
	}

	v, ok := opts[o.key]
	if !ok {
		return o.defaultVal
	}

	var d time.Duration
	d, ok = v.(time.Duration)
	if !ok {
		return o.defaultVal
	}

	return d
}
