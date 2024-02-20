package oncecache

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

// Name is an [Opt] for [New] that sets the cache's name. The name is accessible via [Cache.Name].
//
//	c := oncecache.New[int, string](fetch, oncecache.Name("foobar"))
//
// The name is used by [Cache.String] and [Cache.LogValue]. If [Name] is not
// specified, a random name such as "cache-38a2b7d4" is generated.
type Name string

func (o Name) optioner() {}

// OnFillFunc is a callback [Opt] for [New] that is invoked when a
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
// every [OnEvictFunc] returns. Consider using [OnEvictChan] for long-running
// callbacks.
type OnEvictFunc[K comparable, V any] func(ctx context.Context, c *Cache[K, V], key K, val V, err error)

func (f OnEvictFunc[K, V]) apply(c *Cache[K, V]) {
	c.onEvict = append(c.onEvict, f)
}

// Action is an enumeration of cache actions, as seen on [Event.Action].
type Action uint8

const (
	ActionFill  Action = 1
	ActionEvict Action = 2
)

// String returns action name.
func (a Action) String() string {
	switch a {
	case ActionFill:
		return "fill"
	case ActionEvict:
		return "evict"
	default:
		return "unknown"
	}
}

// Entry is a cache entry, as seen on [Event.Entry].
type Entry[K comparable, V any] struct {
	Cache *Cache[K, V]
	Key   K
	Val   V
	Err   error
}

// Event is a cache event.
type Event[K comparable, V any] struct {
	Entry  Entry[K, V]
	Action Action
}

func OnFillChan[K comparable, V any](ch chan<- Event[K, V], block bool) Opt {
	return eventOpt[K, V]{ch: ch, block: block, action: ActionFill}
}

func OnEvictChan[K comparable, V any](ch chan<- Event[K, V], block bool) Opt {
	return eventOpt[K, V]{ch: ch, block: block, action: ActionEvict}
}

type eventOpt[K comparable, V any] struct {
	ch     chan<- Event[K, V]
	action Action
	block  bool
}

func (o eventOpt[K, V]) optioner() {}

func (o eventOpt[K, V]) apply(c *Cache[K, V]) {
	fn := func(ctx context.Context, c *Cache[K, V], key K, val V, err error) {
		event := Event[K, V]{
			Action: o.action,
			Entry:  Entry[K, V]{Cache: c, Key: key, Val: val, Err: err},
		}

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

	if o.action == ActionFill {
		c.onFill = append(c.onFill, OnFillFunc[K, V](fn))
	} else {
		c.onEvict = append(c.onEvict, OnEvictFunc[K, V](fn))
	}
}
