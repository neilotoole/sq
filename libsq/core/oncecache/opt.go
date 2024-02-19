package oncecache

import "context"

// FetchFunc called by [Cache.Get] to fill an unpopulated cache entry.
type FetchFunc[K comparable, V any] func(ctx context.Context, key K) (val V, err error)

// Opt is a functional option for [New].
type Opt[K comparable, V any] interface {
	apply(c *Cache[K, V])
}

// OnFillFunc is a callback functional option for [New] that is invoked when a
// cache entry is populated, whether on-demand via [Cache.Get] and [FetchFunc],
// or externally via [Cache.Set].
//
// Common use cases include logging, metrics, or cache entry propagation.
//
// Note that the triggering call to [Cache.Set] or [Cache.Get] blocks until
// every [OnFillFunc] returns. Consider spawning a goroutine if the callback is
// long-running.
type OnFillFunc[K comparable, V any] func(ctx context.Context, key K, val V, err error)

func (f OnFillFunc[K, V]) apply(c *Cache[K, V]) {
	c.onFill = append(c.onFill, f)
}

type OptHuzzah[K comparable, V any] struct {
}

func (o OptHuzzah[K, V]) apply(c *Cache[K, V]) {

}

// OnEvictFunc is a callback functional option for [New] that is invoked when a
// cache entry is evicted via [Cache.Delete] or [Cache.Clear].
//
// Common use cases include logging, metrics, or cache entry propagation.
//
// Note that the triggering call to [Cache.Delete] or [Cache.Clear] blocks until
// every [OnEvictFunc] returns. Consider spawning a goroutine if the callback is
// long-running.
type OnEvictFunc[K comparable, V any] func(ctx context.Context, key K, val V, err error)

func (f OnEvictFunc[K, V]) apply(c *Cache[K, V]) {
	c.onEvict = append(c.onEvict, f)
}

type EventType string

const (
	EventFill  = EventType("fill")
	EventEvict = EventType("evict")
)

type Event[K comparable, V any] struct {
	Type  EventType
	Cache *Cache[K, V]
	Key   K
	Val   V
	Err   error
}

//type EventChan[K comparable, V any] chan<- Event[K, V]
//
//func OnFillCallback[K comparable, V any](ch chan<- Event[K, V]) Opt {
//	return onFillOpt[K, V]{ch: ch}
//}
//
//type onFillOpt[K comparable, V any] struct {
//	ch chan<- Event[K, V]
//}
//
//func (o onFillOpt[K, V]) apply() {
//	//TODO implement me
//	panic("implement me")
//}

//func (f onFillOpt[K, V]) apply(c *Cache[K, V]) {
//	//c.onFill = append(c.onFill, f)
//}
