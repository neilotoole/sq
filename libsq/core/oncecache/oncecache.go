// Package oncecache provides a concurrency-safe, in-memory, on-demand cache
// that ensures that a given cache entry is populated only once, unless
// explicitly cleared.
package oncecache

import (
	"context"
	"sync"
)

// New returns a new [Cache] instance. The fetch func is called by [Cache.Get]
// to obtain an entry value for a given key, OR the entry may be externally set
// via [Cache.Set]. Either which way, the entry is populated only once, unless
// it is explicitly cleared via [Cache.Delete] or [Cache.Clear], at which point
// the entry may be populated afresh.
func New[K comparable, V any](fetch func(ctx context.Context, key K) (val V, err error)) *Cache[K, V] {
	return &Cache[K, V]{
		entries: map[K]*entry[K, V]{},
		fetch:   fetch,
	}
}

// Cache is a concurrency-safe, in-memory, on-demand cache that ensures that a
// given cache entry is populated only once (unless explicitly cleared), either
// implicitly via [Cache.Get] or externally via [Cache.Set].
//
// An entry can be explicitly cleared via [Cache.Delete] or [Cache.Clear],
// allowing the entry to be populated afresh.
//
// A cache entry consists not only of the key and value, but also any error
// associated with fetching the value.
//
// The zero value is not usable; instead invoke [New].
type Cache[K comparable, V any] struct {
	fetch   func(ctx context.Context, key K) (val V, err error)
	entries map[K]*entry[K, V]
	mu      sync.Mutex
}

// Clear clears the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
}

// Delete deletes the entry for the given key, allowing the entry to be
// populated afresh via a call to [Cache.Get] or [Cache.Set].
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Set explicitly sets the value and fetch error for the given key, allowing an
// external process to prime the cache. Note that the value can also set
// implicitly via Cache.Get invoking the fetch func. If there's already a cache
// entry for key, Set is no-op: the value is not updated.
func (c *Cache[K, V]) Set(key K, val V, err error) {
	e := c.getEntry(key)
	e.set(val, err)
}

// Get gets the value (and fetch error) for the given key. If there's no entry
// for the key, the fetch func is invoked, setting the entry value and error. If
// the entry is already populated, the value and error are returned without
// invoking the fetch func.
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

	e = &entry[K, V]{fetch: c.fetch}
	c.entries[key] = e
	c.mu.Unlock()
	return e
}

type entry[K comparable, V any] struct {
	val   V
	err   error
	fetch func(ctx context.Context, key K) (val V, err error)
	once  sync.Once
}

func (e *entry[K, V]) set(val V, err error) {
	e.once.Do(func() {
		e.val = val
		e.err = err
	})
}

func (e *entry[K, V]) get(ctx context.Context, key K) (V, error) {
	e.once.Do(func() {
		e.val, e.err = e.fetch(ctx, key)
	})
	return e.val, e.err
}
