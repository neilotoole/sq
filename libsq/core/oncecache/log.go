package oncecache

import (
	"context"
	"fmt"
	"log/slog"
)

// Log is an [Opt] for [oncecache.New] that logs cache events to log. If log is
// nil, Log is a no-op. If lvl is nil, [slog.LevelInfo] is used.
func Log[K comparable, V any](log *slog.Logger, lvl slog.Leveler, ops ...Op) Opt {
	if log == nil {
		return nil
	}
	if isNil(lvl) {
		lvl = slog.LevelInfo
	}

	if len(ops) == 0 {
		ops = []Op{OpHit, OpMiss, OpFill, OpEvict}
	} else {
		ops = uniq(ops)
	}

	o := &logOpt[K, V]{
		log: log,
		lvl: lvl,
		ops: ops,
	}

	return o
}

type logOpt[K comparable, V any] struct {
	log *slog.Logger
	lvl slog.Leveler
	ops []Op
}

func (o *logOpt[K, V]) optioner() {}

func (o *logOpt[K, V]) apply(c *Cache[K, V]) {
	for _, op := range o.ops {
		switch op {
		case OpFill:
			c.onFill = append(c.onFill, o.logFill)
		case OpEvict:
			c.onEvict = append(c.onEvict, o.logEvict)
		case OpHit:
			c.onHit = append(c.onHit, o.logHit)
		case OpMiss:
			c.onMiss = append(c.onMiss, o.logMiss)
		default:
			// Shouldn't happen.
			panic(fmt.Sprintf("oncecache: unknown op[%d]: %s", op, op))
		}
	}
}

func (o *logOpt[K, V]) logEvent(ctx context.Context, ev Event[K, V]) {
	o.log.LogAttrs(ctx, o.lvl.Level(), LogMessage, slog.Any(LogAttrKey, ev))
}

func (o *logOpt[K, V]) logHit(ctx context.Context, key K, val V, err error) {
	ev := Event[K, V]{
		Op:    OpHit,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

func (o *logOpt[K, V]) logMiss(ctx context.Context, key K, val V, err error) {
	ev := Event[K, V]{
		Op:    OpMiss,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

func (o *logOpt[K, V]) logFill(ctx context.Context, key K, val V, err error) {
	ev := Event[K, V]{
		Op:    OpFill,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}
func (o *logOpt[K, V]) logEvict(ctx context.Context, key K, val V, err error) {
	ev := Event[K, V]{
		Op:    OpEvict,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

// LogEvents logs cache events from ch to the given logger. It continues until ch is
// consumed or the non-nil ctx is done. It is common to spawn a goroutine to
// handle the logging. For example:
//
//	eventCh := make(chan oncecache.Event[int, int], 100)
//	c := oncecache.New[int, int](
//	  calcFibonacci,
//	  oncecache.Name("fibs"),
//	  oncecache.OnEvent(eventCh, true),
//	)
//
//	go oncecache.LogEvents(ctx, eventCh, log, slog.LevelDebug, nil)
//
//	c.Get(ctx, 10) // emits miss, then fill
//	c.Get(ctx, 10) // emits hit
//
//	// Output:
//	level=DEBUG msg="Cache event" e.op=miss e.cache=fibs e.key=10 e.val=0 e.err=<nil>
//	level=DEBUG msg="Cache event" e.op=fill e.cache=fibs e.key=10 e.val=55 e.err=<nil>
//	level=DEBUG msg="Cache event" e.op=hit e.cache=fibs e.key=10 e.val=55 e.err=<nil>
//
// Note that [LogEvents] returns immediately if log or ch are nil. If lvl is nil,
// [slog.LevelInfo] is used.
//
// If msgFmt is non-nil, it is invoked on each event to format the log message;
// otherwise, [oncecache.LogMessage] is used.
//
// Note that [Event.LogValue] implements [slog.LogValuer]; the event is logged
// as an attribute using [oncecache.LogAttrKey].
func LogEvents[K comparable, V any](ctx context.Context, ch <-chan Event[K, V],
	log *slog.Logger, lvl slog.Leveler, msgFmt func(Event[K, V]) string,
) {
	if ch == nil || log == nil {
		return
	}

	if isNil(lvl) {
		lvl = slog.LevelInfo
	}

	msg := LogMessage

	if ctx == nil {
		for e := range ch {
			if msgFmt != nil {
				msg = msgFmt(e)
			}
			log.Log(nil, lvl.Level(), msg, slog.Any(LogAttrKey, e)) //nolint:staticcheck
		}
		return
	}

	done := ctx.Done()
	for ctx.Err() == nil {
		select {
		case e, ok := <-ch:
			if !ok {
				return
			}
			if msgFmt != nil {
				msg = msgFmt(e)
			}

			log.LogAttrs(ctx, lvl.Level(), msg, slog.Any(LogAttrKey, e))
		case <-done:
			return
		}
	}
}

var (
	// LogMessage is the default log message used by [LogEvents] if a custom message
	// formatter is not provided.
	LogMessage = "Cache event"

	// LogAttrKey is the slog attribute key used by [oncecache.LogEvents] to log events.
	LogAttrKey = "e"
)
