// Package oncecache provides a strongly-typed, concurrency-safe, in-memory,
// on-demand cache that ensures that a given cache entry is populated only once.
package oncecache

import (
	"context"
	"sync"
)

// New returns a new [Cache] instance. The fetch func is called, on-demand, by
// [Cache.Get] to obtain an entry value for a given key, OR the entry may be
// externally set via [Cache.Set]. Either which way, the entry is populated only
// once. That is, unless the entry is explicitly cleared via [Cache.Delete] or
// [Cache.Clear], at which point the entry may be populated afresh.
//
// The opts are functional options that can be used to configure the cache. For
// example, see the [OnFillFunc] or [OnEvictFunc] callbacks.
func New[K comparable, V any](fetch FetchFunc[K, V], opts ...Opt) *Cache[K, V] {
	c := &Cache[K, V]{
		entries: map[K]*entry[K, V]{},
		fetch:   fetch,
	}

	for _, opt := range opts {
		opt.apply()
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
	onFill  []OnFillFunc[K, V]
	onEvict []OnEvictFunc[K, V]
	entries map[K]*entry[K, V]
	mu      sync.Mutex
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
	for key, ce := range c.entries {
		if ce != nil {
			ce := ce
			evictions = append(evictions, func() { ce.evict(ctx, key) })
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
func (c *Cache[K, V]) Delete(ctx context.Context, key interface{}) {
	c.mu.Lock()
	ce, ok := c.entries[key]
	delete(c.entries, key)
	c.mu.Unlock()
	if ok && ce != nil {
		ce.evict(ctx, key)
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

	if notify {
		for _, onFill := range e.cache.onFill {
			onFill(ctx, key, val, err)
		}
	}
}

func (e *entry[K, V]) get(ctx context.Context, key K) (V, error) {
	var notify bool
	e.once.Do(func() {
		e.val, e.err = e.cache.fetch(ctx, key)
		notify = true
	})

	if notify {
		for _, onFill := range e.cache.onFill {
			onFill(ctx, key, e.val, e.err)
		}
	}

	return e.val, e.err
}

func (e *entry[K, V]) evict(ctx context.Context, key K) {
	for _, onEvict := range e.cache.onEvict {
		onEvict(ctx, key, e.val, e.err)
	}
}

