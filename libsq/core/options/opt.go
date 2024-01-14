package options

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	// TagSource indicates that an Opt with this tag applies to source config
	// (as opposed to applying only to base config). An opt with this tag
	// typically applies to both base and source config.
	TagSource = "source"

	// TagTuning indicates that the Opt is related to tuning behavior.
	TagTuning = "tuning"

	// TagSQL indicates that the Opt is related to SQL interaction.
	TagSQL = "sql"

	// TagOutput indicates the Opt is related to output/formatting.
	TagOutput = "output"

	// TagIngestMutate indicates the Opt may result in mutated data, particularly
	// during ingestion. This tag is significant in that its value may affect
	// data realization, and thus affect program aspects such as caching behavior.
	TagIngestMutate = "mutate"
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

	// Flag is the long flag name to use, which is typically the same value
	// as returned by Opt.Key. However, a distinct value can be supplied, such
	// that flag usage and config usage have different keys. For example,
	// an Opt might have a key "diff.num-lines", but a flag "lines".
	Flag() string

	// Short is the short key. The zero value indicates no short key.
	// For example, if the key is "json", the short key could be 'j'.
	Short() rune

	// Usage is a one-line description of the Opt. Additional detail can be
	// found in Help.
	Usage() string

	// Help returns the Opt's help text, which typically provides more detail
	// than Usage. The text must be plaintext (not markdown). Linebreaks are
	// recommended at 100 chars.
	Help() string

	// String returns a log/debug-friendly representation.
	String() string

	// IsSet returns true if this Opt is set in o.
	IsSet(o Options) bool

	// GetAny returns the value of this Opt in o. Generally, prefer
	// use of the concrete strongly-typed Get method. If o is nil or
	// empty, or the Opt is not in o, the Opt's default value is
	// returned.
	GetAny(o Options) any

	// DefaultAny returns the default value of this Opt. Generally, prefer
	// use of the concrete strongly-typed Default method.
	DefaultAny() any

	// Tags returns any tags on this Opt instance. For example, an Opt might
	// have tags [source, csv].
	Tags() []string

	// HasTag returns true if the result of Opt.Tags contains tag.
	HasTag(tag string) bool

	// Process processes o. The returned Options may be a new instance,
	// with mutated values. This is typ
	Process(o Options) (Options, error)
}

// BaseOpt is a partial implementation of options.Opt that concrete
// types can build on.
type BaseOpt struct {
	key   string
	flag  string
	short rune
	usage string
	help  string
	tags  []string
}

// NewBaseOpt returns a new BaseOpt. If flag is empty string, key is
// used as the flag value.
func NewBaseOpt(key, flag string, short rune, usage, help string, tags ...string) BaseOpt {
	if flag == "" {
		flag = key
	}

	slices.Sort(tags)

	return BaseOpt{
		key:   key,
		flag:  flag,
		short: short,
		usage: usage,
		help:  help,
		tags:  tags,
	}
}

// Key implements options.Opt.
func (op BaseOpt) Key() string {
	return op.key
}

// Flag implements options.Opt.
func (op BaseOpt) Flag() string {
	return op.flag
}

// Short implements options.Opt.
func (op BaseOpt) Short() rune {
	return op.short
}

// Usage implements options.Opt.
func (op BaseOpt) Usage() string {
	return op.usage
}

// Help implements options.Opt.
func (op BaseOpt) Help() string {
	return op.help
}

// IsSet implements options.Opt.
func (op BaseOpt) IsSet(o Options) bool {
	if o == nil {
		return false
	}

	return o.IsSet(op)
}

// GetAny is required by options.Opt. It needs to be implemented
// by the concrete type.
func (op BaseOpt) GetAny(_ Options) any {
	panic(fmt.Sprintf("GetAny not implemented for: %s", op.key))
}

// DefaultAny implements options.Opt.
func (op BaseOpt) DefaultAny() any {
	panic(fmt.Sprintf("DefaultAny not implemented for: %s", op.key))
}

// String implements options.Opt.
func (op BaseOpt) String() string {
	return op.key
}

// Tags implements options.Opt.
func (op BaseOpt) Tags() []string {
	return op.tags
}

// HasTag implements options.Opt.
func (op BaseOpt) HasTag(tag string) bool {
	return slices.Contains(op.tags, tag)
}

// Process implements options.Opt.
func (op BaseOpt) Process(o Options) (Options, error) {
	return o, nil
}

var _ Opt = String{}

// NewString returns an options.String instance. If flag is empty, the
// value of key is used. If valid Fn is non-nil, it is called from
// the process function.
//
//nolint:revive
func NewString(key, flag string, short rune, defaultVal string,
	validFn func(string) error, usage, help string, tags ...string,
) String {
	return String{
		BaseOpt:    NewBaseOpt(key, flag, short, usage, help, tags...),
		defaultVal: defaultVal,
		validFn:    validFn,
	}
}

// String is an options.Opt for type string.
type String struct {
	BaseOpt
	defaultVal string
	validFn    func(string) error
}

// GetAny implements options.Opt.
func (op String) GetAny(o Options) any {
	return op.Get(o)
}

// DefaultAny implements options.Opt.
func (op String) DefaultAny() any {
	return op.defaultVal
}

// Default returns the default value of op.
func (op String) Default() string {
	return op.defaultVal
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

// Process implements options.Opt. If the String was constructed
// with validator function, it is invoked on the value of the Opt,
// if it is set. Otherwise the method is no-op.
func (op String) Process(o Options) (Options, error) {
	if op.validFn == nil {
		return o, nil
	}

	v, ok := o[op.key]
	if !ok || v == nil {
		return o, nil
	}

	var s string
	if s, ok = v.(string); !ok {
		return nil, errz.Errorf("expected string value for {%s} but got %T: %v", op.key, v, v)
	}

	if err := op.validFn(s); err != nil {
		return nil, err
	}

	return o, nil
}

var _ Opt = Int{}

// NewInt returns an options.Int instance. If flag is empty, the
// value of key is used.
func NewInt(key, flag string, short rune, defaultVal int, usage, help string, tags ...string) Int {
	return Int{
		BaseOpt:    NewBaseOpt(key, flag, short, usage, help, tags...),
		defaultVal: defaultVal,
	}
}

// Int is an options.Opt for type int.
type Int struct {
	BaseOpt
	defaultVal int
}

// Default returns the default value of op.
func (op Int) Default() int {
	return op.defaultVal
}

// GetAny implements options.Opt.
func (op Int) GetAny(o Options) any {
	return op.Get(o)
}

// DefaultAny implements options.Opt.
func (op Int) DefaultAny() any {
	return op.defaultVal
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

// NewBool returns an options.Bool instance. If flag is empty, the value
// of key is used. If invertFlag is true, the flag's boolean value
// is inverted to set the option. For example, if the Opt is "progress",
// and the flag is "--no-progress", then invertFlag should be true.
func NewBool(key, flag string, invertFlag bool, short rune, //nolint:revive
	defaultVal bool, usage, help string, tags ...string,
) Bool {
	return Bool{
		BaseOpt:      NewBaseOpt(key, flag, short, usage, help, tags...),
		defaultVal:   defaultVal,
		flagInverted: invertFlag,
	}
}

// Bool is an options.Opt for type bool.
type Bool struct {
	BaseOpt
	defaultVal   bool
	flagInverted bool
}

// FlagInverted returns true Opt value is the inverse of the flag value.
// For example, if the Opt is "progress", and the flag is "--no-progress",
// then FlagInverted will return true.
func (op Bool) FlagInverted() bool {
	return op.flagInverted
}

// GetAny implements options.Opt.
func (op Bool) GetAny(opts Options) any {
	return op.Get(opts)
}

// DefaultAny implements options.Opt.
func (op Bool) DefaultAny() any {
	return op.defaultVal
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

// Default returns the default value of op.
func (op Bool) Default() bool {
	return op.defaultVal
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

// NewDuration returns an options.Duration instance. If flag is empty, the
// value of key is used.
func NewDuration(key, flag string, short rune, defaultVal time.Duration,
	usage, help string, tags ...string,
) Duration {
	return Duration{
		BaseOpt:    NewBaseOpt(key, flag, short, usage, help, tags...),
		defaultVal: defaultVal,
	}
}

// Duration is an options.Opt for time.Duration.
type Duration struct {
	BaseOpt
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

// Default returns the default value of op.
func (op Duration) Default() time.Duration {
	return op.defaultVal
}

// DefaultAny implements options.Opt.
func (op Duration) DefaultAny() any {
	return op.defaultVal
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
