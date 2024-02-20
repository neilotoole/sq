// Package oncecache contains a strongly-typed, concurrency-safe, context-aware,
// dependency-free, in-memory, on-demand object [Cache], focused on fill-once,
// read-many ergonomics.
//
// The package also provides an event mechanism useful for linked cache
// propagation, logging, or metrics.
package oncecache

import (
	"context"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"log/slog"
	"reflect"
	"sync"
)

// FetchFunc called by [Cache.Get] to fill an unpopulated cache entry. If
// needed, the source [Cache] can be retrieved from ctx via [FromContext].
type FetchFunc[K comparable, V any] func(ctx context.Context, key K) (val V, err error)

// New returns a new [Cache] instance. The fetch func is invoked, on-demand, by
// [Cache.Get] to obtain an entry value for a given key, OR the entry may be
// externally set via [Cache.Set]. Either which way, the entry is populated only
// once. That is, unless the entry is explicitly cleared via [Cache.Delete] or
// [Cache.Clear], at which point the entry may be populated afresh.
//
// Arg opts is a set of functional options that can be used to configure the
// cache. For example, see [Name] to set the cache name, or the [OnFillFunc]
// or [OnEvictFunc] callbacks.
func New[K comparable, V any](fetch FetchFunc[K, V], opts ...Opt) *Cache[K, V] {
	c := &Cache[K, V]{
		name:    randomName(),
		entries: map[K]*entry[K, V]{},
		fetch:   fetch,
	}

	for _, opt := range opts {
		if !isNil(opt) {
			if optioner, ok := opt.(optApplier[K, V]); ok {
				optioner.apply(c)
				continue
			}

			// Else, we've got to do it case-by-case.
			if name, ok := opt.(Name); ok {
				c.name = string(name)
				continue
			}
		}
	}

	return c
}

// Cache is a concurrency-safe, in-memory, on-demand cache that ensures that a
// given cache entry is populated only once (unless explicitly cleared), either
// implicitly via [Cache.Get] or externally via [Cache.Set].
//
// An entry can be explicitly cleared via [Cache.Delete] or [Cache.Clear],
// allowing the entry to be populated afresh.
//
// A cache entry consists not only of the key and value, but also any error
// associated with setting the entry value.
//
// The zero value is not usable; instead invoke [New].
type Cache[K comparable, V any] struct {
	fetch   FetchFunc[K, V]
	entries map[K]*entry[K, V]
	name    string
	onFill  []OnFillFunc[K, V]
	onEvict []OnEvictFunc[K, V]
	mu      sync.Mutex
}

// Name returns the cache's name, useful for logging. Specify the cache name by
// passing [oncecache.Name] to [New]; otherwise a random name is used.
func (c *Cache[K, V]) Name() string {
	return c.name
}

// Len returns the number of entries in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// String returns a debug-friendly string representation of the cache.
func (c *Cache[K, V]) String() string {
	return fmt.Sprintf(
		"%s[%T, %T][%d]",
		c.name,
		*new(K),
		*new(V),
		c.Len(),
	)
}

// LogValue implements [slog.LogValuer].
func (c *Cache[K, V]) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("name", c.name),
		slog.Int("entries", c.Len()),
		slog.Group("type",
			"key", fmt.Sprintf("%T", *new(K)),
			"val", fmt.Sprintf("%T", *new(V)),
		),
	)
}

// Clear clears the cache entries, invoking any [OnEvictFunc] callbacks on each
// cache entry. The entry callback order is not specified.
func (c *Cache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()

	if len(c.onEvict) == 0 {
		clear(c.entries)
		c.mu.Unlock()
		return
	}

	var evictions []func()
	for key, ent := range c.entries {
		if ent != nil {
			e := ent
			evictions = append(evictions, func() { e.evict(ctx, key) })
		}
	}

	clear(c.entries)
	c.mu.Unlock()

	for _, evict := range evictions {
		evict()
	}
}

// Delete deletes the entry for the given key, invoking any [OnEvictFunc]
// callbacks.
func (c *Cache[K, V]) Delete(ctx context.Context, key K) {
	c.mu.Lock()
	ce, ok := c.entries[key]
	delete(c.entries, key)
	c.mu.Unlock()
	if ok && ce != nil {
		ce.evict(newContext(ctx, c), key)
	}
}

// Set explicitly sets the value and fetch error for the given key, allowing an
// external process to prime the cache. Note that the value can alternatively be
// filled implicitly via [Cache.Get] invoking the fetch func. If there's already
// a cache entry for key, Set is no-op: the value is not updated. If this Set
// call does update the cache entry, any [OnFillFunc] callbacks (as provided to
// [New]) are invoked.
func (c *Cache[K, V]) Set(ctx context.Context, key K, val V, err error) {
	e := c.getEntry(key)
	e.set(ctx, key, val, err)
}

// Get gets the value (and fill error) for the given key. If there's no entry
// for the key, the fetch func is invoked, setting the entry value and error. If
// the entry is already populated, the value and error are returned without
// invoking the fetch func. If population does occur, any [OnFillFunc] callbacks
// (as provided to [New]) are invoked, and this call blocks until all callbacks
// return.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	e := c.getEntry(key)
	return e.get(ctx, key)
}

func (c *Cache[K, V]) getEntry(key K) *entry[K, V] {
	c.mu.Lock()
	e, ok := c.entries[key]
	if ok {
		c.mu.Unlock()
		return e
	}

	e = &entry[K, V]{cache: c}
	c.entries[key] = e
	c.mu.Unlock()
	return e
}

// Opt is an option for [New].
type Opt interface {
	optioner()
}

// optApplier is an Opt that uses the apply method to modify a Cache instance.
type optApplier[K comparable, V any] interface {
	Opt
	apply(c *Cache[K, V])
}

// Name is an [Opt] for [New] that sets the cache's name. The name is accessible via [Cache.Name].
//
//	c := oncecache.New[int, string](fetch, oncecache.Name("foobar"))
//
// The name is used by [Cache.String] and [Cache.LogValue]. If [Name] is not
// specified, a random name such as "cache-38a2b7d4" is generated.
type Name string

func (o Name) optioner() {}

type entry[K comparable, V any] struct {
	val   V
	err   error
	cache *Cache[K, V]
	once  sync.Once
}

func (e *entry[K, V]) set(ctx context.Context, key K, val V, err error) {
	var notify bool
	e.once.Do(func() {
		e.val = val
		e.err = err
		notify = true
	})

	if notify && len(e.cache.onFill) > 0 {
		ctx = newContext(ctx, e.cache)
		for _, onFill := range e.cache.onFill {
			onFill(ctx, e.cache, key, val, err)
		}
	}
}

func (e *entry[K, V]) get(ctx context.Context, key K) (V, error) {
	var notify bool
	e.once.Do(func() {
		ctx = newContext(ctx, e.cache)
		e.val, e.err = e.cache.fetch(ctx, key)
		notify = true
	})

	if notify && len(e.cache.onFill) > 0 {
		for _, onFill := range e.cache.onFill {
			onFill(ctx, e.cache, key, e.val, e.err)
		}
	}

	return e.val, e.err
}

// evict invokes any [OnEvictFunc] callbacks for the given cache entry. The
// supplied ctx should already be decorated via newContext.
func (e *entry[K, V]) evict(ctx context.Context, key K) {
	for _, onEvict := range e.cache.onEvict {
		onEvict(ctx, e.cache, key, e.val, e.err)
	}
}

func randomName() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	return fmt.Sprintf("cache-%x", crc32.ChecksumIEEE(b))
}

type ctxKey struct{}

// NewContext returns ctx with c added as a value. If ctx is nil, a new context
// is created.
func newContext[K comparable, V any](ctx context.Context, c *Cache[K, V]) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the [Cache] value stored in ctx, if any, or nil. All
// cache callbacks receive a context that has been decorated with the [Cache]
// instance.
func FromContext[K comparable, V any](ctx context.Context) *Cache[K, V] {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(ctxKey{})
	if val == nil {
		return nil
	}

	if c, ok := val.(*Cache[K, V]); ok {
		return c
	}

	return nil
}

// isNil checks if a value is nil or if it's a reference type with a nil underlying value.
func isNil(x any) bool {
	defer func() { recover() }() //nolint:errcheck
	return x == nil || reflect.ValueOf(x).IsNil()
}
