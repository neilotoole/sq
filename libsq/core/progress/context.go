package progress

import (
	"context"
)

type progCtxKey struct{}

// NewContext returns ctx with p added as a value.
func NewContext(ctx context.Context, p *Progress) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, progCtxKey{}, p)
}

// FromContext returns the Progress added to ctx via NewContext, or returns nil.
// Note that it is safe to invoke the methods of a nil Progress.
func FromContext(ctx context.Context) *Progress {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(progCtxKey{})
	if val == nil {
		return nil
	}

	if p, ok := val.(*Progress); ok {
		return p
	}

	return nil
}

type barCtxKey struct{}

// NewBarContext returns ctx with bar added as a value. This context can be used
// in conjunction with progress.Incr to increment the progress bar.
func NewBarContext(ctx context.Context, bar Bar) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, barCtxKey{}, bar)
}

// Incr invokes [Bar.Incr] with amount n on the outermost [Bar] in ctx. Use in
// conjunction with a context returned from NewBarContext. It safe to invoke
// Incr on a nil context or a context that doesn't contain a Bar.
//
// NOTE: This context-based incrementing is a bit of an experiment. I'm hesitant
// in going further with context-based logic, as it's not clear to me that it's
// a good idea to lean on context so much. So, it's possible this mechanism may
// be removed in the future.
func Incr(ctx context.Context, n int) {
	if ctx == nil {
		return
	}

	val := ctx.Value(barCtxKey{})
	if val == nil {
		return
	}

	if b, ok := val.(Bar); ok {
		b.Incr(n)
	}
}
