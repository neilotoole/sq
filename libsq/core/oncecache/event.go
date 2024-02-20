package oncecache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Entry is the external representation of a cache entry. It is not part of the
// cache's internal state; it can be modified by the user if desired.
type Entry[K comparable, V any] struct {
	Cache *Cache[K, V]
	Key   K
	Val   V
	Err   error
}

// String returns a string representation of the entry.
func (e Entry[K, V]) String() string {
	sb := strings.Builder{}
	sb.WriteString(e.Cache.name)
	sb.WriteRune('[')
	sb.WriteString(fmt.Sprintf("%v", e.Key))
	sb.WriteRune(']')
	if e.Err != nil {
		sb.WriteString("[! ")
		sb.WriteString(e.Err.Error())
		sb.WriteRune(']')
	}
	val := fmt.Sprintf("%v", e.Val)
	if len(val) > 32 {
		sb.WriteString(val[:13])
		sb.WriteString("...")
		sb.WriteString(val[len(val)-13:])
	} else {
		sb.WriteString(val)
	}
	return sb.String()
}

// LogValue implements slog.LogValuer.
func (e Entry[K, V]) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("cache", e.Cache.name),
		slog.Any("key", e.Key),
		slog.Any("val", e.Val),
		slog.Any("err", e.Err),
	)
}

// Event is a cache event.
type Event[K comparable, V any] struct {
	Entry[K, V]
	Action Action
}

// LogValue implements slog.LogValuer.
func (e Event[K, V]) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("action", e.Action.String()),
		slog.String("cache", e.Cache.name),
		slog.Any("key", e.Key),
		slog.Any("val", e.Val),
		slog.Any("err", e.Err),
	)
}

// String returns a string representation of the event.
func (e Event[K, V]) String() string {
	var sb strings.Builder
	sb.WriteString(e.Action.String())
	sb.WriteString(": ")
	sb.WriteString(e.Cache.name)
	sb.WriteRune('[')
	sb.WriteString(fmt.Sprintf("%v", e.Key))
	sb.WriteRune(']')
	if e.Err != nil {
		sb.WriteString("[! ")
		sb.WriteString(e.Err.Error())
		sb.WriteRune(']')
	}
	val := fmt.Sprintf(" %v", e.Val)
	if len(val) > 32 {
		sb.WriteString(val[:14])
		sb.WriteString("...")
		sb.WriteString(val[len(val)-14:])
	} else {
		sb.WriteString(val)
	}
	return sb.String()
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
	fn := func(ctx context.Context, key K, val V, err error) {
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

// OnFillFunc is a callback [Opt] for [New] that is invoked when a
// cache entry is populated, whether on-demand via [Cache.Get] and [FetchFunc],
// or externally via [Cache.Set].
//
// Common use cases include logging, metrics, or cache entry propagation.
//
// Note that the triggering call to [Cache.Set] or [Cache.Get] blocks until
// every [OnFillFunc] returns. Consider using [OnFillChan] for long-running
// callbacks.
type OnFillFunc[K comparable, V any] func(ctx context.Context, key K, val V, err error)

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
type OnEvictFunc[K comparable, V any] func(ctx context.Context, key K, val V, err error)

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
