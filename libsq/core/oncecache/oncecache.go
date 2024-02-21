// Package oncecache contains a strongly-typed, concurrency-safe, context-aware,
// dependency-free, in-memory, on-demand object [Cache], focused on write-once,
// read-often ergonomics.
//
// The package also provides an event mechanism useful for logging, metrics, or
// propagating cache entries between overlapping composite caches.
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

// FetchFunc is called by [Cache.Get] to fill an unpopulated cache entry. If
// needed, the source [Cache] can be retrieved from ctx via [FromContext].
type FetchFunc[K comparable, V any] func(ctx context.Context, key K) (val V, err error)

// New returns a new [Cache] instance. The fetch func is invoked, on-demand, by
// [Cache.Get] to obtain an entry value for a given key, OR the entry may be
// externally set via [Cache.Set]. Either which way, the entry is populated only
// once. That is, unless the entry is explicitly cleared via [Cache.Delete] or
// [Cache.Clear], at which point the entry may be populated afresh.
//
// Arg opts is a set of options that can be used to configure the cache. For
// example, see [Name] to set the cache name, or the [OnFill] or [OnEvict]
// options for event callbacks.
func New[K comparable, V any](fetch FetchFunc[K, V], opts ...Opt) *Cache[K, V] {
	c := &Cache[K, V]{
		name:    randomName(),
		entries: map[K]*entry[K, V]{},
		fetch:   fetch,
	}

	c.applyOpts(opts)
	return c
}

// applyOpts applies functional options.
func (c *Cache[K, V]) applyOpts(opts []Opt) {
	for _, opt := range opts {
		if isNil(opt) {
			continue
		}

		// Most [Opt] types implement [optApplier]...
		if applier, ok := opt.(optApplier[K, V]); ok {
			if _, ok = opt.(concreteOptApplier); ok {
				// Sanity check.
				panic(fmt.Sprintf("Opt type %T must not implement both optApplier and concreteOptApplier", opt))
			}
			applier.apply(c)
			continue
		}

		// But, not all of them. Some of them implement [concreteOptApplier]. We
		// must provide the non-parameterized (concrete) fields of [Cache].
		npc := &concreteCache{name: &c.name}
		if applier, ok := opt.(concreteOptApplier); ok {
			applier.applyConcrete(npc)
			continue
		}

		// Else, something went badly wrong.
		panic(fmt.Sprintf("Invalid Opt type %T", opt))
	}
}

// Cache is a concurrency-safe, in-memory, on-demand cache that ensures that a
// given cache entry is populated only once, either implicitly via [Cache.Get]
// and the fetch func, or externally via [Cache.Set].
//
// However, a cache entry can be explicitly cleared via [Cache.Delete] or
// [Cache.Clear], allowing the entry to be populated afresh.
//
// A cache entry consists not only of the key and value, but also any error
// associated with filling the entry value via the fetch func or via
// [Cache.Set]. Thus, a cache entry is a triple: (key, value, error). An entry
// with a non-nil error is still a valid cache entry. A call to [Cache.Get] for
// an existing errorful cache entry does not invoke the fetch func again. Cache
// entry population occurs only once (hence "oncecache"), unless the entry is
// explicitly evicted via [Cache.Delete] or [Cache.Clear].
//
// The zero value is not usable; instead invoke [New].
type Cache[K comparable, V any] struct {
	fetch   FetchFunc[K, V]
	entries map[K]*entry[K, V]
	name    string
	onFill  []callbackFunc[K, V]
	onEvict []callbackFunc[K, V]
	onHit   []callbackFunc[K, V]
	onMiss  []callbackFunc[K, V]
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
	return fmt.Sprintf("%s[%T, %T][%d]",
		c.name, *new(K), *new(V), c.Len(),
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

// Has returns true if [Cache] c has an entry for key.
func (c *Cache[K, V]) Has(key K) bool {
	c.mu.Lock()
	_, ok := c.entries[key]
	c.mu.Unlock()
	return ok
}

// Clear clears the cache entries, invoking any [OnEvict] callbacks on each
// cache entry. The entry callback order is not specified.
func (c *Cache[K, V]) Clear(ctx context.Context) {
	c.mu.Lock()

	if len(c.onEvict) == 0 {
		clear(c.entries)
		c.mu.Unlock()
		return
	}

	evictions := make([]func(), 0, len(c.entries))
	ctx = NewContext(ctx, c)
	for key, ent := range c.entries {
		if ent == nil {
			continue // Shouldn't be possible
		}
		e := ent
		evictions = append(evictions, func() { e.notifyEvict(ctx, key) })
	}

	clear(c.entries)
	for _, fn := range evictions {
		fn()
	}
	c.mu.Unlock()

}

// Delete deletes the entry for the given key, invoking any [OnEvict]
// callbacks.
func (c *Cache[K, V]) Delete(ctx context.Context, key K) {
	c.mu.Lock()
	e, ok := c.entries[key]
	if ok {
		delete(c.entries, key)
		if e != nil {
			e.notifyEvict(NewContext(ctx, c), key)
		}
	}
	c.mu.Unlock()
}

// Set explicitly sets the value and fill error for the given key, allowing an
// external process to prime the cache. Note that the value can alternatively be
// filled implicitly via [Cache.Get], when it invokes the fetch func. If there's
// already a cache entry for key, Set is no-op: the value is not updated. If
// this Set call does update the cache entry, any [OnFill] callbacks - as
// provided to [New] - are invoked.
func (c *Cache[K, V]) Set(ctx context.Context, key K, val V, err error) {
	e := c.getEntry(key)
	e.set(ctx, key, val, err)
}

// Get gets the value (and fill error) for the given key. If there's no entry
// for the key, the fetch func is invoked, setting the entry value and error. If
// the entry is already populated, the value and error are returned without
// invoking the fetch func. If population does occur, any [OnFill] callbacks -
// as provided to [New] - are invoked, and this Get call blocks until all
// callbacks return.
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
	// optioner is a marker method to unify our two functional option types,
	// optApplier and concreteOptApplier.
	optioner()
}

// optApplier is an [Opt] that uses the apply method to configure the fields of
// [Cache]. It must be type-parameterized, as this Opt access the parameterized
// fields of [Cache].
type optApplier[K comparable, V any] interface {
	Opt
	apply(c *Cache[K, V])
}

// concreteOptApplier is an [Opt] type that uses the applyConcrete method to
// configure the non-parameterized (concrete) fields of [Cache].
//
// TODO: Write a post about this pattern:
// "Mixing concrete and type-parameterized functional options".
type concreteOptApplier interface {
	Opt
	applyConcrete(c *concreteCache)
}

// concreteCache contains pointers to the non-parameterized (concrete) state of
// [Cache]. It is passed to concreteOptApplier.applyConcrete by [New].
type concreteCache struct {
	name *string
}

var _ concreteOptApplier = (*Name)(nil)

// Name is an [Opt] for [New] that sets the cache's name. The name is accessible
// via [Cache.Name].
//
//	c := oncecache.New[int, string](fetch, oncecache.Name("foobar"))
//
// The name is used by [Cache.String] and [Cache.LogValue]. If [Name] is not
// specified, a random name such as "cache-38a2b7d4" is generated.
type Name string

func (o Name) applyConcrete(c *concreteCache) {
	*c.name = string(o)
}

func (o Name) optioner() {}

// entry is the internal representation of a cache entry. Contrast with the
// external [Entry] type.
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

	// We perform notification outside the once to avoid holding the lock.
	if notify && len(e.cache.onFill) > 0 {
		ctx = NewContext(ctx, e.cache)
		for _, fn := range e.cache.onFill {
			fn(ctx, key, val, err)
		}
	}
}

func (e *entry[K, V]) get(ctx context.Context, key K) (V, error) {
	var miss bool
	e.once.Do(func() {
		miss = true
		ctx = NewContext(ctx, e.cache)
		for _, fn := range e.cache.onMiss {
			fn(ctx, key, e.val, e.err)
		}

		e.val, e.err = e.cache.fetch(ctx, key)

		for _, fn := range e.cache.onFill {
			fn(ctx, key, e.val, e.err)
		}
	})

	if !miss && len(e.cache.onHit) > 0 {
		ctx = NewContext(ctx, e.cache)
		for _, fn := range e.cache.onHit {
			fn(ctx, key, e.val, e.err)
		}
	}

	return e.val, e.err
}

// notifyEvict invokes any [OnEvict] callbacks for the given cache entry. The
// caller should beforehand decorate ctx via [NewContext].
func (e *entry[K, V]) notifyEvict(ctx context.Context, key K) {
	for _, fn := range e.cache.onEvict {
		fn(ctx, key, e.val, e.err)
	}
}

func randomName() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	return fmt.Sprintf("cache-%x", crc32.ChecksumIEEE(b))
}

type ctxKey struct{}

// NewContext returns ctx decorated with [Cache] c. If ctx is nil, a new context
// is created.
func NewContext[K comparable, V any](ctx context.Context, c *Cache[K, V]) context.Context {
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

// isNil checks if a value is nil or if it's a reference type with a nil
// underlying value.
func isNil(x any) bool {
	defer func() { recover() }() //nolint:errcheck
	return x == nil || reflect.ValueOf(x).IsNil()
}
