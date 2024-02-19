package ocache

import "context"

// Opt is an option for [New].
type Opt interface {
	optioner()
}

// optApplier is an Opt that uses the apply method to modify a Cache instance.
type optApplier[K comparable, V any] interface {
	Opt
	apply(c *Cache[K, V])
}

// Name is a functional option for [New] that sets the cache's name, as
// accessible via [Cache.Name].
type Name string

func (o Name) optioner() {}

//func (o Name[K, V]) apply(c *Cache[K, V]) {
//	c.name = string(o)
//}

// OnFillFunc is a callback functional option for [New] that is invoked when a
// cache entry is populated, whether on-demand via [Cache.Get] and [FetchFunc],
// or externally via [Cache.Set].
//
// Common use cases include logging, metrics, or cache entry propagation.
//
// Note that the triggering call to [Cache.Set] or [Cache.Get] blocks until
// every [OnFillFunc] returns. Consider using [OnFillChan] for long-running
// callbacks.
type OnFillFunc[K comparable, V any] func(ctx context.Context, c *Cache[K, V], key K, val V, err error)

func (f OnFillFunc[K, V]) apply(c *Cache[K, V]) {
	c.onFill = append(c.onFill, f)
}

// OnEvictFunc is a callback functional option for [New] that is invoked when a
// cache entry is evicted via [Cache.Delete] or [Cache.Clear].
//
// Common use cases include logging, metrics, or cache entry propagation.
//
// Note that the triggering call to [Cache.Delete] or [Cache.Clear] blocks until
// every [OnEvictFunc] returns. Consider spawning a goroutine if the callback is
// long-running.
type OnEvictFunc[K comparable, V any] func(ctx context.Context, c *Cache[K, V], key K, val V, err error)

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

func OnFillChan[K comparable, V any](ch chan<- Event[K, V], block bool) Opt {
	return eventOpt[K, V]{ch: ch, block: block, typ: EventFill}
}
func OnEvictChan[K comparable, V any](ch chan<- Event[K, V], block bool) Opt {
	return eventOpt[K, V]{ch: ch, block: block, typ: EventFill}
}

type eventOpt[K comparable, V any] struct {
	typ   EventType
	block bool
	ch    chan<- Event[K, V]
}

func (o eventOpt[K, V]) optioner() {}

func (o eventOpt[K, V]) apply(c *Cache[K, V]) {
	fn := func(ctx context.Context, c *Cache[K, V], key K, val V, err error) {
		event := Event[K, V]{Type: o.typ, Cache: c, Key: key, Val: val, Err: err}

		if o.block {
			// Blocking.
			o.ch <- event
			return
		}

		// Non-blocking.
		select {
		case o.ch <- event:
		default:
		}
	}

	if o.typ == EventFill {
		c.onFill = append(c.onFill, OnFillFunc[K, V](fn))
	} else {
		c.onEvict = append(c.onEvict, OnEvictFunc[K, V](fn))
	}
}

//type EventChan[K comparable, V any] chan<- Event[K, V]
//
//func OnFillCallback[K comparable, V any](ch chan<- Event[K, V]) Opt {
//	return eventOpt[K, V]{ch: ch}
//}
//
//type eventOpt[K comparable, V any] struct {
//	ch chan<- Event[K, V]
//}
//
//func (o eventOpt[K, V]) apply() {
//	//TODO implement me
//	panic("implement me")
//}

//func (f eventOpt[K, V]) apply(c *Cache[K, V]) {
//	//c.onFill = append(c.onFill, f)
//}
