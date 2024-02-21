package oncecache

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// The LogX variables are used by [oncecache.Log] via [oncecache.Event.LogValue]
// to control cache event log output.
var (
	LogMsg       = "Cache event"
	LogAttrEvent = "ev"
	LogAttrCache = "cache"
	LogAttrOp    = "op"
	LogAttrKey   = "k"
	LogAttrVal   = "v"
	LogAttrErr   = "err"
)

// Log is an [Opt] for [oncecache.New] that logs each [Event] to log, where
// [Event.Op] is in ops. If ops is empty, all events are logged. If log is nil,
// Log is a no-op. If lvl is nil, [slog.LevelInfo] is used.
//
// Note that [Event.LogValue] implements [slog.LogValuer]. Output is controlled
// by the LogX variables, e.g. [LogMsg], [LogAttrEvent], etc.
//
// If you require even more control, you can roll your own logging mechanism
// using an [OnEvent] channel or the On* callbacks.
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

func (o *logOpt[K, V]) apply(c *Cache[K, V]) { //nolint:unused // linter wrong
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

func (o *logOpt[K, V]) logEvent(ctx context.Context, ev Event[K, V]) { //nolint:unused // linter wrong
	o.log.LogAttrs(ctx, o.lvl.Level(), LogMsg, slog.Any(LogAttrEvent, ev))
}

func (o *logOpt[K, V]) logHit(ctx context.Context, key K, val V, err error) { //nolint:unused // linter wrong
	ev := Event[K, V]{
		Op:    OpHit,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

func (o *logOpt[K, V]) logMiss(ctx context.Context, key K, val V, err error) { //nolint:unused // linter wrong
	ev := Event[K, V]{
		Op:    OpMiss,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

func (o *logOpt[K, V]) logFill(ctx context.Context, key K, val V, err error) { //nolint:unused // linter wrong
	ev := Event[K, V]{
		Op:    OpFill,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

func (o *logOpt[K, V]) logEvict(ctx context.Context, key K, val V, err error) { //nolint:unused // linter is wrong
	ev := Event[K, V]{
		Op:    OpEvict,
		Entry: Entry[K, V]{Cache: FromContext[K, V](ctx), Key: key, Val: val, Err: err},
	}
	o.logEvent(ctx, ev)
}

// LogValue implements [slog.LogValuer], logging according to [Entry.LogValue],
// but also logging [Event.Op].
func (e Event[K, V]) LogValue() slog.Value {
	attrs := make([]slog.Attr, 3, 5)
	attrs[0] = slog.String(LogAttrCache, e.Cache.name)
	attrs[1] = slog.String(LogAttrOp, e.Op.String())
	attrs[2] = slog.Any(LogAttrKey, e.Key)

	if e.Op != OpMiss {
		if e.isValLogged() {
			attrs = append(attrs, slog.Any(LogAttrVal, e.Val))
		}
		if e.Err != nil {
			attrs = append(attrs, slog.Any(LogAttrErr, e.Err))
		}
	}

	return slog.GroupValue(attrs...)
}

// String returns a string representation of the event. The event's Val field
// is not incorporated. For logging, note [Event.LogValue].
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
	return sb.String()
}

// String returns a string representation of the entry. The entry's Val field
// is not incorporated. For logging, note [Entry.LogValue].
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
	return sb.String()
}

// LogValue implements [slog.LogValuer], logging Val if it implements
// [slog.LogValuer] or is a primitive type such as int or bool (but not string),
// logging Err if non-nil, and always logging Key and [Cache.Name].
func (e Entry[K, V]) LogValue() slog.Value {
	attrs := make([]slog.Attr, 2, 4)
	attrs[0] = slog.String(LogAttrCache, e.Cache.name)
	attrs[1] = slog.Any(LogAttrKey, e.Key)

	if e.isValLogged() {
		attrs = append(attrs, slog.Any(LogAttrVal, e.Val))
	}
	if e.Err != nil {
		attrs = append(attrs, slog.Any(LogAttrErr, e.Err))
	}

	return slog.GroupValue(attrs...)
}

// isValLogged returns true if the entry's Val field should be logged. Only
// primitive types and [slog.LogValuer] are logged. In particular, note that
// string is not logged.
func (e Entry[K, V]) isValLogged() bool {
	switch any(e.Val).(type) {
	case slog.LogValuer, bool, nil, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, complex64, complex128:
		return true
	default:
		return false
	}
}
