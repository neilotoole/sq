package oncecache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/samber/lo"
)

type callbackFunc[K comparable, V any] func(ctx context.Context, key K, val V, err error)

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
	val := fmt.Sprintf(" = %v", e.Val)
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
	Op Op
}

// LogValue implements slog.LogValuer.
func (e Event[K, V]) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("op", e.Op.String()),
		slog.String("cache", e.Cache.name),
		slog.Any("key", e.Key),
		slog.Any("val", e.Val),
		slog.Any("err", e.Err),
	)
}

// String returns a string representation of the event.
func (e Event[K, V]) String() string {
	var sb strings.Builder

	sb.WriteString(e.Cache.name)
	sb.WriteRune('.')
	sb.WriteString(e.Op.String())
	sb.WriteRune('[')
	sb.WriteString(fmt.Sprintf("%v", e.Key))
	sb.WriteRune(']')
	if e.Err != nil {
		sb.WriteString("[! ")
		sb.WriteString(e.Err.Error())
		sb.WriteRune(']')
	}
	val := fmt.Sprintf(" = %v", e.Val)
	if len(val) > 32 {
		sb.WriteString(val[:14])
		sb.WriteString("...")
		sb.WriteString(val[len(val)-14:])
	} else {
		sb.WriteString(val)
	}
	return sb.String()
}

// OnEvent is an [Opt] argument to [New] that configures the cache to emit
// events on the given chan. If ops is empty, all events are emitted; otherwise,
// only events for the given ops are emitted.
//
// If arg block is true, the [Cache] function that triggered the event will
// block on sending on a full ch. If false, the new event is dropped if ch is
// full.
//
// You can use an unbuffered channel and block=true to stop the event consumer
// from falling too far behind the cache state. Alternatively the synchronous
// [OnHit], [OnMiss], [OnFill], and [OnEvict] callbacks can be used, at cost of
// increased lock contention and lower throughput.
func OnEvent[K comparable, V any](ch chan<- Event[K, V], block bool, ops ...Op) Opt {
	ops = lo.Uniq(ops)
	if len(ops) == 0 {
		ops = []Op{OpFill, OpEvict, OpHit, OpMiss}
	}

	return eventOpt[K, V]{ch: ch, block: block, ops: lo.Uniq(ops)}
}

type eventOpt[K comparable, V any] struct {
	ch    chan<- Event[K, V]
	ops   []Op
	block bool
}

func (o eventOpt[K, V]) optioner() {}

func (o eventOpt[K, V]) apply(c *Cache[K, V]) { //nolint:unused // linter is wrong, method is invoked.
	for _, op := range o.ops {
		op := op
		fn := func(ctx context.Context, key K, val V, err error) {
			event := Event[K, V]{
				Op:    op,
				Entry: Entry[K, V]{Cache: c, Key: key, Val: val, Err: err},
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

		switch op {
		case OpFill:
			c.onFill = append(c.onFill, fn)
		case OpEvict:
			c.onEvict = append(c.onEvict, fn)
		case OpHit:
			c.onHit = append(c.onHit, fn)
		case OpMiss:
			c.onMiss = append(c.onMiss, fn)
		default:
			// Shouldn't happen.
			panic(fmt.Sprintf("unknown action: %v: %s", op, op))
		}
	}
}

// onEventFuncOpt is [Opt] type returned by [OnFill], [OnEvict], [OnHit], and
// [OnMiss].
type onEventFuncOpt[K comparable, V any] struct {
	fn callbackFunc[K, V]
	op Op
}

func (f onEventFuncOpt[K, V]) optioner() {}

func (f onEventFuncOpt[K, V]) apply(c *Cache[K, V]) { //nolint:unused // linter is wrong, method is invoked.
	switch f.op {
	case OpFill:
		c.onFill = append(c.onFill, f.fn)
	case OpEvict:
		c.onEvict = append(c.onEvict, f.fn)
	case OpHit:
		c.onHit = append(c.onHit, f.fn)
	case OpMiss:
		c.onMiss = append(c.onMiss, f.fn)
	default:
		// Shouldn't happen.
		panic(fmt.Sprintf("unknown op: %v: %s", f.op, f.op))
	}
}

// OnFill returns a callback [Opt] for [New] that is invoked when a cache entry
// is populated, whether on-demand via [Cache.Get] and [FetchFunc], or
// externally via [Cache.MaybeSet].
//
// Note that [OnFill] callbacks are synchronous; the triggering call to
// [Cache.MaybeSet] or [Cache.Get] blocks until every [OnFill] returns. Consider
// using [OnEvent] for long-running callbacks.
//
// While [OnFill] can be used for logging, metrics, etc., most common tasks are
// better accomplished via [OnEvent].
func OnFill[K comparable, V any](fn func(ctx context.Context, key K, val V, err error)) Opt {
	return onEventFuncOpt[K, V]{op: OpFill, fn: fn}
}

// OnEvict returns a callback [Opt] for [New] that is invoked when a cache entry
// is evicted via [Cache.Delete] or [Cache.Clear].
//
// Note that [OnEvict] callbacks are synchronous; the triggering call to
// [Cache.Delete] or [Cache.Clear] blocks until every [OnEvict] returns.
// Consider using [OnEvent] for long-running callbacks.
func OnEvict[K comparable, V any](fn func(ctx context.Context, key K, val V, err error)) Opt {
	return onEventFuncOpt[K, V]{op: OpEvict, fn: fn}
}

// OnHit returns a callback [Opt] for [New] that is invoked when [Cache.Get]
// results in a cache hit.
//
// Note that [OnHit] callbacks are synchronous; the triggering call to
// [Cache.Get] blocks until every [OnHit] returns. Consider using the
// asynchronous [OnEvent] for long-running callbacks.
func OnHit[K comparable, V any](fn func(ctx context.Context, key K, val V, err error)) Opt {
	return onEventFuncOpt[K, V]{op: OpHit, fn: fn}
}

// OnMiss returns a callback [Opt] for [New] that is invoked when [Cache.Get]
// results in a cache miss.
//
// Note that [OnMiss] callbacks are synchronous; the triggering call to
// [Cache.Get] blocks until every [OnMiss] returns. Consider using the
// asynchronous [OnEvent] for long-running callbacks.
func OnMiss[K comparable, V any](fn func(ctx context.Context, key K)) Opt {
	return onEventFuncOpt[K, V]{op: OpMiss, fn: func(ctx context.Context, key K, val V, err error) {
		fn(ctx, key)
	}}
}

// Op is an enumeration of cache operations, as see in [Event.Op].
type Op uint8

const (
	// OpHit indicates a cache hit: a cache entry already exists for the key. Note
	// that the cache entry may contain a non-nil error, and the entry value may
	// be the zero value. An errorful cache entry is a valid hit.
	OpHit Op = 1

	// OpMiss indicates a cache miss. It is always immediately followed by an
	// [OpFill].
	OpMiss Op = 2

	// OpFill indicates that a cache entry has been populated. Typically it is
	// immediately preceded by [OpMiss], but will occur standalone when
	// [Cache.Set] is invoked. Note that if the entry fill results in an error,
	// the entry is still considered valid, and [OpFill] is still emitted.
	OpFill Op = 3

	// OpEvict indicates a cache entry has been removed.
	OpEvict Op = 4
)

// IsZero returns true if the action is the zero value, which is an invalid Op.
func (o Op) IsZero() bool {
	return o == 0
}

// String returns the op name.
func (o Op) String() string {
	switch o {
	case OpFill:
		return "fill"
	case OpEvict:
		return "evict"
	case OpHit:
		return "hit"
	case OpMiss:
		return "miss"
	default:
		return "unknown"
	}
}
