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
// a "Get(o Options) T" method that returns the appropriate type T. The
// value returned by that Get method will be the same as that returned
// by the generic Opt.GetAny method. The impl should also provide a NewT(...) T
// constructor. The caller typically registers the new Opt in a options.Registry
// via Registry.Add.
//
// An impl should implement the Process method to appropriately munge the
// backing value. For example, options.Duration converts a
// string such as "1m30s" into a time.Duration.
type Opt interface {
	// Key returns the Opt key, such as "ping.timeout".
	Key() string

	// Comment returns the Opt's comment.
	Comment() string

	// String returns a log/debug friendly representation.
	String() string

	// IsSet returns true if this Opt is set in o.
	IsSet(o Options) bool

	// GetAny returns the value of this Opt in o. Generally, prefer
	// use of the concrete strongly-typed Get method. If o is nil or
	// empty, or the Opt is not in o, the Opt's default value is
	// returned.
	GetAny(o Options) any

	// Tags returns any tags on this Opt instance. For example, an Opt might
	// have tags [source, csv].
	Tags() []string

	// Process processes o. The returned Options may be a new instance,
	// with mutated values. This is typ
	Process(o Options) (Options, error)
}

type baseOpt struct {
	key     string
	comment string
	tags    []string
}

// Key implements options.Opt.
func (op baseOpt) Key() string {
	return op.key
}

// Comment implements options.Opt.
func (op baseOpt) Comment() string {
	return op.comment
}

// IsSet implements options.Opt.
func (op baseOpt) IsSet(o Options) bool {
	if o == nil {
		return false
	}

	return o.IsSet(op)
}

// GetAny is required by options.Opt. It needs to be implemented
// by the concrete type.
func (op baseOpt) GetAny(_ Options) any {
	panic("not implemented")
}

// String implements options.Opt.
func (op baseOpt) String() string {
	return op.key
}

// Tags implements options.Opt.
func (op baseOpt) Tags() []string {
	return op.tags
}

// Process implements options.Opt.
func (op baseOpt) Process(o Options) (Options, error) {
	return o, nil
}

var _ Opt = String{}

// NewString returns an options.String instance.
func NewString(key, defaultVal, comment string, tags ...string) String {
	return String{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
}

// String is an options.Opt for type string.
type String struct {
	baseOpt
	defaultVal string
}

// GetAny implements options.Opt.
func (op String) GetAny(o Options) any {
	return op.Get(o)
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op String) Get(o Options) string {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.key]
	if !ok {
		return op.defaultVal
	}

	var s string
	s, ok = v.(string)
	if !ok {
		return op.defaultVal
	}

	return s
}

var _ Opt = Int{}

// NewInt returns an options.Int instance.
func NewInt(key string, defaultVal int, comment string, tags ...string) Int {
	return Int{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
}

// Int is an options.Opt for type int.
type Int struct {
	baseOpt
	defaultVal int
}

// GetAny implements options.Opt.
func (op Int) GetAny(o Options) any {
	return op.Get(o)
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op Int) Get(o Options) int {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return op.defaultVal
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
		return op.defaultVal
	}
}

// Process implements options.Opt. It converts matching
// values in o into bool. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op Int) Process(o Options) (Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return o, nil
	}

	if _, ok = v.(int); ok {
		// Happy path
		return o, nil
	}

	o = o.Clone()

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
			delete(o, op.key)
			return o, nil
		}

		var err error
		if i, err = strconv.Atoi(v); err != nil {
			return nil, errz.Wrapf(err, "invalid int value for {%s}: %v", op.key, v)
		}
	default:
		// This shouldn't happen, but it's a last-ditch effort.
		// Print v as a string, and try to parse it.
		s := fmt.Sprintf("%v", v)
		var err error
		if i, err = strconv.Atoi(s); err != nil {
			return nil, errz.Wrapf(err, "invalid int value for {%s}: %v", op.key, v)
		}
	}

	o[op.key] = i
	return o, nil
}

var _ Opt = Bool{}

// NewBool returns an options.Bool instance.
func NewBool(key string, defaultVal bool, comment string, tags ...string) Bool {
	return Bool{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
}

// Bool is an options.Opt for type bool.
type Bool struct {
	baseOpt
	defaultVal bool
}

// GetAny implements options.Opt.
func (op Bool) GetAny(opts Options) any {
	return op.Get(opts)
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op Bool) Get(o Options) bool {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return op.defaultVal
	}

	var b bool
	b, ok = v.(bool)
	if !ok {
		return op.defaultVal
	}

	return b
}

// Process implements options.Opt. It converts matching
// string values in o into bool. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op Bool) Process(o Options) (Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return o, nil
	}

	if _, ok = v.(bool); ok {
		// Happy path
		return o, nil
	}

	o = o.Clone()

	switch v := v.(type) {
	case string:
		if v == "" {
			// Empty string is effectively nil
			delete(o, op.key)
			return o, nil
		}

		// It could be a string like "true"
		b, err := stringz.ParseBool(v)
		if err != nil {
			return nil, errz.Wrapf(err, "invalid bool value for {%s}: %v", op.key, v)
		}
		o[op.key] = b
	default:
		// Well, we don't know what this is... maybe a number like "1"?
		// Last-ditch effort. Print the value to a string, and check
		// if we can parse the string into a bool.
		s := fmt.Sprintf("%v", v)
		b, err := stringz.ParseBool(s)
		if err != nil {
			return nil, errz.Wrapf(err, "invalid bool value for {%s}: %v", op.key, v)
		}
		o[op.key] = b
	}

	return o, nil
}

var _ Opt = Duration{}

// NewDuration returns an options.Duration instance.
func NewDuration(key string, defaultVal time.Duration, comment string, tags ...string) Duration {
	return Duration{
		baseOpt:    baseOpt{key: key, comment: comment, tags: tags},
		defaultVal: defaultVal,
	}
}

// Duration is an options.Opt for time.Duration.
type Duration struct {
	baseOpt
	defaultVal time.Duration
}

// Process implements options.Opt. It converts matching
// string values in o into time.Duration. If no match found,
// the input arg is returned unchanged. Otherwise, a clone is
// returned.
func (op Duration) Process(o Options) (Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return o, nil
	}

	if _, ok = v.(time.Duration); ok {
		// v is already a duration, nothing to do here.
		return o, nil
	}

	// v should be a string
	var s string
	s, ok = v.(string)
	if !ok {
		return nil, errz.Errorf("option {%s} should be {%T} but got {%T}: %v",
			op.key, s, v, v)
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, errz.Wrapf(err, "options {%s} is not a valid duration", op.key)
	}

	o = o.Clone()
	o[op.key] = d
	return o, nil
}

// GetAny implements options.Opt.
func (op Duration) GetAny(o Options) any {
	return op.Get(o)
}

// Get returns op's value in o. If o is nil, or no value
// is set, op's default value is returned.
func (op Duration) Get(o Options) time.Duration {
	if o == nil {
		return op.defaultVal
	}

	v, ok := o[op.key]
	if !ok {
		return op.defaultVal
	}

	var d time.Duration
	d, ok = v.(time.Duration)
	if !ok {
		return op.defaultVal
	}

	return d
}
