// Package oncecache contains a strongly-typed, concurrency-safe, context-aware,
// dependency-free, in-memory, on-demand object [Cache], focused on write-once,
// read-often ergonomics.
//
// The package also provides an event mechanism useful for logging, metrics, or
// propagating cache entries between overlapping composite caches.
package oncecache

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"log/slog"
	"reflect"
	"sync"

	"golang.org/x/exp/maps"
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
// options for event callbacks. Any nil [Opt] in opts is ignored.
func New[K comparable, V any](fetch FetchFunc[K, V], opts ...Opt) *Cache[K, V] {
	c := &Cache[K, V]{
		name:    randomName(),
		entries: map[K]*entry[K, V]{},
		fetch:   fetch,
	}

	c.applyOpts(opts)

	if len(c.onFill)+len(c.onMiss)+len(c.onHit) == 0 {
		c.getValueFn = getValueFast[K, V]
	} else {
		c.getValueFn = getValueSlow[K, V]
	}

	if len(c.onFill) == 0 {
		c.maybeSetValueFn = maybeSetValueFast[K, V]
	} else {
		c.maybeSetValueFn = maybeSetValueSlow[K, V]
	}

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
		cc := &concreteCache{name: &c.name}
		if applier, ok := opt.(concreteOptApplier); ok {
			applier.applyConcrete(cc)
			continue
		}

		// Hmmmn, I thought I was soooo clever with that applyConcrete mechanism,
		// but it isn't helping with the logOpt scenario. Rethink required.
		if cfg, ok := opt.(logOptConfig); ok {
			(*logOpt[K, V])(&cfg).apply(c)
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
	fetch           FetchFunc[K, V]
	entries         map[K]*entry[K, V]
	maybeSetValueFn func(ctx context.Context, e *entry[K, V], key K, val V, err error) bool
	getValueFn      func(ctx context.Context, e *entry[K, V], key K) (V, error)
	name            string
	onFill          []callbackFunc[K, V]
	onEvict         []callbackFunc[K, V]
	onHit           []callbackFunc[K, V]
	onMiss          []callbackFunc[K, V]
	mu              sync.Mutex
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

// Keys returns the cache keys. The keys will be in an indeterminate order.
func (c *Cache[K, V]) Keys() []K {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := make([]K, 0, len(c.entries))
	for k := range c.entries {
		r = append(r, k)
	}
	return r
}

// Clear clears the cache entries, invoking any [OnEvict] callbacks on each
// cache entry. The entry callback order is not specified. The cache is locked
// until Clear (including any callbacks) returns.
func (c *Cache[K, V]) Clear(ctx context.Context) {
	if len(c.onEvict) == 0 {
		c.mu.Lock()
		clear(c.entries)
		c.mu.Unlock()
		return
	}

	ctx = NewContext(ctx, c)
	c.mu.Lock()
	for key, e := range c.entries {
		delete(c.entries, key)
		if e == nil {
			continue // Shouldn't be possible
		}

		for _, fn := range e.cache.onEvict {
			fn(ctx, key, e.val, e.err)
		}
	}
	c.mu.Unlock()
}

// Delete deletes the entry for the given key, invoking any [OnEvict] callbacks.
// The cache is locked until Delete (including any callbacks) returns.
func (c *Cache[K, V]) Delete(ctx context.Context, key K) {
	if len(c.onEvict) == 0 {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return
	}

	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return
	}

	delete(c.entries, key)
	ctx = NewContext(ctx, c)
	for _, fn := range e.cache.onEvict {
		fn(ctx, key, e.val, e.err)
	}
	c.mu.Unlock()
}

// MaybeSet sets the value and fill error for the given key if it is not already
// filled, returning true if the value was set. This would allow an external
// process to prime the cache.
//
// Note that the value might instead be filled implicitly via [Cache.Get], when
// it invokes the fetch func. If there's already a cache entry for key, MaybeSet
// is no-op: the value is not updated. If this MaybeSet call does update the
// cache entry, any [OnFill] callbacks - as provided to [New] - are invoked, and
// ok returns true.
func (c *Cache[K, V]) MaybeSet(ctx context.Context, key K, val V, err error) (ok bool) {
	e := c.getEntry(key)
	return c.maybeSetValueFn(ctx, e, key, val, err)
}

// Get gets the value (and fill error) for the given key. If there's no entry
// for the key, the fetch func is invoked, setting the entry value and error. If
// the entry is already populated, the value and error are returned without
// invoking the fetch func. Any [OnHit], [OnMiss], and [OnFill] callbacks are
// invoked, and [OpHit], [OpMiss] and [OpFill] events emitted, as appropriate.
func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, error) {
	e := c.getEntry(key)
	return c.getValueFn(ctx, e, key)
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

// Close closes the cache, releasing any resources. Close is idempotent and
// always returns nil. Callbacks are not invoked. The cache is not usable after
// Close is invoked; calls to other [Cache] methods may panic.
func (c *Cache[K, V]) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.onFill = nil
	c.onEvict = nil
	c.onHit = nil
	c.onMiss = nil
	clear(c.entries)
	return nil
}

var (
	_ gob.GobEncoder = (*Cache[int, int])(nil)
	_ gob.GobDecoder = (*Cache[int, int])(nil)
)

type gobData[K comparable, V any] struct {
	Entries map[K]*gobEntry[K, V]
	Name    string
}

type gobEntry[K comparable, V any] struct {
	Val V
	Err error
}

// GobEncode implements [gob.GobEncoder]. Only the cache name and entries are
// encoded. The fetch func and callbacks are not encoded.
func (c *Cache[K, V]) GobEncode() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	gbd := &gobData[K, V]{
		Name:    c.name,
		Entries: make(map[K]*gobEntry[K, V], len(c.entries)),
	}

	for k, ent := range c.entries {
		gbd.Entries[k] = &gobEntry[K, V]{Val: ent.val, Err: ent.err}
	}

	buf := &bytes.Buffer{}
	if err := gob.NewEncoder(buf).Encode(gbd); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GobDecode implements [gob.GobDecoder]. Only the cache name and entries are
// decoded. The fetch func and callbacks are not decoded. Any pre-existing
// entries in c are cleared prior to decoding.
func (c *Cache[K, V]) GobDecode(p []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	gbd := &gobData[K, V]{}
	if err := gob.NewDecoder(bytes.NewReader(p)).Decode(gbd); err != nil {
		return err
	}

	c.name = gbd.Name
	maps.Clear(c.entries)
	for k, e := range gbd.Entries {
		ent := &entry[K, V]{val: e.Val, err: e.Err, cache: c}
		ent.once.Do(func() {}) // Consume the sync.Once
		c.entries[k] = ent
	}

	return nil
}

// entry is the internal representation of a cache entry. Contrast with the
// external [Entry] type.
type entry[K comparable, V any] struct {
	val   V
	err   error
	cache *Cache[K, V]
	once  sync.Once
}

func maybeSetValueSlow[K comparable, V any](ctx context.Context, e *entry[K, V], key K, val V, err error) bool {
	var ok bool
	e.once.Do(func() {
		ok = true
		e.val = val
		e.err = err
		ctx = NewContext(ctx, e.cache)
		for _, fn := range e.cache.onFill {
			fn(ctx, key, val, err)
		}
	})
	return ok
}

func maybeSetValueFast[K comparable, V any](_ context.Context, e *entry[K, V], _ K, val V, err error) bool {
	var ok bool
	e.once.Do(func() {
		e.val = val
		e.err = err
		ok = true
	})
	return ok
}

func getValueSlow[K comparable, V any](ctx context.Context, e *entry[K, V], key K) (V, error) {
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

func getValueFast[K comparable, V any](ctx context.Context, e *entry[K, V], key K) (V, error) {
	e.once.Do(func() {
		ctx = NewContext(ctx, e.cache)
		e.val, e.err = e.cache.fetch(ctx, key)
	})

	return e.val, e.err
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

func randomName() string {
	b := make([]byte, 128)
	_, _ = rand.Read(b)
	return fmt.Sprintf("cache-%x", crc32.ChecksumIEEE(b))
}

// isNil checks if a value is nil or if it's a reference type with a nil
// underlying value.
func isNil(x any) bool {
	defer func() { recover() }() //nolint:errcheck
	return x == nil || reflect.ValueOf(x).IsNil()
}

// uniq returns a new slice containing only the unique elements of a.
func uniq[T comparable](a []T) []T {
	result := make([]T, 0, len(a))
	seen := make(map[T]struct{}, len(a))

	for _, val := range a {
		if _, ok := seen[val]; ok {
			continue
		}

		seen[val] = struct{}{}
		result = append(result, val)
	}

	return result
}
