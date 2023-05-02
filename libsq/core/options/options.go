// Package options implements config options. This package is currently a bit
// of an experiment. Objectives:
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
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/exp/slog"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

type contextKey struct{}

// NewContext returns a context that contains the given Options.
// Use FromContext to retrieve the Options.
//
// NOTE: It's questionable whether we need to engage in this context
// business with Options. This is a bit of an experiment.
func NewContext(ctx context.Context, o Options) context.Context {
	return context.WithValue(ctx, contextKey{}, o)
}

// FromContext returns the Options stored in ctx by NewContext, or nil
// if no such Options.
func FromContext(ctx context.Context) Options {
	v := ctx.Value(contextKey{})
	if v == nil {
		return nil
	}

	if v, ok := v.(Options); ok {
		return v
	}

	return nil
}

// Registry is a registry of Opt instances.
type Registry struct {
	mu   sync.Mutex
	opts []Opt
}

// Add adds opts to r. It panics if any element of opts is already registered.
func (r *Registry) Add(opts ...Opt) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, opt := range opts {
		for i := range r.opts {
			if r.opts[i].Key() == opt.Key() {
				panic(fmt.Sprintf("Opt %s is already registered", opt.Key()))
			}
		}

		r.opts = append(r.opts, opt)
	}
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

// Visit visits each Opt in r. Be careful with concurrent access
// via this method.
func (r *Registry) Visit(fn func(opt Opt) error) error {
	if r == nil {
		return nil
	}

	for i := range r.opts {
		if err := fn(r.opts[i]); err != nil {
			return err
		}
	}
	return nil
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

// Process processes o, returning a new Options. Process should be invoked
// after the Options has been loaded from config, but before it is used by the
// program. Process iterates over the registered Opts, and invokes Process for
// each Opt that implements Processor. This facilitates munging of backing
// values, e.g. for options.Duration, a string "1m30s" is converted to a time.Duration.
func (r *Registry) Process(o Options) (Options, error) {
	if o == nil {
		return nil, nil //nolint:nilnil
	}

	opts := r.opts
	o2 := Options{}
	for _, opt := range opts {
		if v, ok := o[opt.Key()]; ok {
			o2[opt.Key()] = v
		}
	}

	var err error
	for _, opt := range opts {
		if o2, err = opt.Process(o2); err != nil {
			return nil, err
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

// LogValue implements slog.LogValuer.
func (o Options) LogValue() slog.Value {
	if o == nil {
		return slog.Value{}
	}

	attrs := make([]slog.Attr, 0, len(o))
	for k, v := range o {
		switch v := v.(type) {
		case int:
			attrs = append(attrs, slog.Int(k, v))
		case string:
			attrs = append(attrs, slog.String(k, v))
		case bool:
			attrs = append(attrs, slog.Bool(k, v))
		case time.Duration:
			attrs = append(attrs, slog.Duration(k, v))
		default:
			attrs = append(attrs, slog.Any(k, v))
		}
	}

	return slog.GroupValue(attrs...)
}

// Merge overlays each of overlays onto base, returning a new Options.
func Merge(base Options, overlays ...Options) Options {
	o := base.Clone()
	for _, overlay := range overlays {
		for k, v := range overlay {
			o[k] = v
		}
	}
	return o
}

// Effective returns a new Options containing the effective values
// of each Opt. That is to say, the returned Options contains either
// the actual value of each Opt in o, or the default value for that Opt,
// but it will not contain values for any Opt not in opts.
func Effective(o Options, opts ...Opt) Options {
	o2 := Options{}
	for _, opt := range opts {
		v := opt.GetAny(o)
		o2[opt.Key()] = v
	}
	return o2
}

// Processor performs processing on o.
type Processor interface {
	// Process processes o. The returned Options may be a new instance,
	// with mutated values.
	Process(o Options) (Options, error)
}

// DeleteNil deletes any keys with nil values.
func DeleteNil(o Options) Options {
	if o == nil {
		return nil
	}

	o = o.Clone()
	for k, v := range o {
		if v == nil {
			delete(o, k)
		}
	}

	return o
}
