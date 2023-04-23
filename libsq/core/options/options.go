// Package options implements config options. This package is currently a bit
// of an experiment. Objectives:
//
//   - Options are key-value pairs.
//   - Options can come from default config, individual source config, and flags.
//   - Support the ability to edit config in $EDITOR, providing contextual information
//     about the Opt instance.
//   - Values are strongly typed (e.g. int, time.Duration)
//   - An individual Opt instance can be specified near where it is used.
//   - New types of Opt can be defined, near where they are used.
//
// It is noted that these requirements could probably largely be met using
// packages such as spf13/viper. AGain, this is largely an experiment.
package options

import (
	"fmt"
	"sync"

	"golang.org/x/exp/slog"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// DefaultRegistry is the default Registry. The constructor functions
// for each concrete options.Opt type should invoke Registry.Add on
// options.DefaultRegistry.
var DefaultRegistry = &Registry{}

// Registry is a registry of Opt instances.
type Registry struct {
	mu   sync.Mutex
	opts []Opt
}

// Add adds an Opt to r. It panics if opt is already registered.
func (r *Registry) Add(opt Opt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.opts {
		if r.opts[i].Key() == opt.Key() {
			panic(fmt.Sprintf("Opt %s is already registered", opt.Key()))
		}
	}

	r.opts = append(r.opts, opt)
}

// LogValue implements slog.LogValuer.
func (r *Registry) LogValue() slog.Value {
	r.mu.Lock()
	defer r.mu.Unlock()
	as := make([]slog.Attr, len(r.opts))
	for i, opt := range r.opts {
		as[i] = slog.String(opt.Key(), fmt.Sprintf("%T", opt))
	}
	return slog.GroupValue(as...)
}

// Get returns the Opt registered in r using key, or nil.
func (r *Registry) Get(key string) Opt {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, opt := range r.opts {
		if opt.Key() == key {
			return opt
		}
	}
	return nil
}

// Keys returns the keys of each Opt in r.
func (r *Registry) Keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	keys := make([]string, len(r.opts))
	for i := range r.opts {
		keys[i] = r.opts[i].Key()
	}
	return keys
}

// Opts returns a new slice containing each Opt registered with r.
func (r *Registry) Opts() []Opt {
	r.mu.Lock()
	defer r.mu.Unlock()

	opts := make([]Opt, len(r.opts))
	copy(opts, r.opts)
	return opts
}

// Process implements options.Processor. It processes arg options, returning a
// new Options. Process should be invoked after the Options has been loaded from
// config, but before it is used by the program. Process iterates over the
// registered Opts, and invokes Process for each Opt that implements Processor.
// This facilitates munging of underlying values, e.g. for options.Duration, a
// string "1m30s" is converted to a time.Duration.
func (r *Registry) Process(options Options) (Options, error) {
	return process(options, r.Opts())
}

func process(options Options, opts []Opt) (Options, error) {
	if options == nil {
		return nil, errz.New("options is nil")
	}

	o2 := Options{}
	for _, opt := range opts {
		if v, ok := options[opt.Key()]; ok {
			o2[opt.Key()] = v
		}
	}

	var err error
	for _, o := range opts {
		if n, ok := o.(Processor); ok {
			if o2, err = n.Process(o2); err != nil {
				return nil, err
			}
		}
	}

	return o2, nil
}

// Options is a map of Opt.Key to a value.
type Options map[string]any

// Clone clones o.
func (o Options) Clone() Options {
	if o == nil {
		return nil
	}

	o2 := Options{}
	for k, v := range o {
		o2[k] = v
	}

	return o2
}

// Keys returns the sorted set of keys in o.
func (o Options) Keys() []string {
	keys := lo.Keys(o)
	slices.Sort(keys)
	return keys
}

// IsSet returns true if opt is set on o.
func (o Options) IsSet(opt Opt) bool {
	_, ok := o[opt.Key()]
	return ok
}

// Processor performs processing on o.
type Processor interface {
	// Process processes o. The returned Options may be a new instance,
	// with mutated values.
	Process(o Options) (Options, error)
}
