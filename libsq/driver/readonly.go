package driver

import "context"

// readOnlyCtxKey is the (unexported) context key carrying the read-only hint.
// The stored bool reports whether the hint is explicit: true when the user
// asked for read-only directly (e.g. sq sql --readonly), false when a
// non-writing command (sq, inspect, diff, ping) set the hint implicitly.
type readOnlyCtxKey struct{}

// WithReadOnly returns a derived context that signals to driver.Driver
// implementations that the caller does not intend to write to the source.
// Drivers that can honor this hint (currently: DuckDB) should connect in
// read-only mode; drivers that cannot must ignore it.
//
// The hint is advisory. A driver that ignores it MUST still produce a
// working connection. The hint is implicit: it never overrides an access
// mode the user set on the source location. For explicit user intent
// (the --readonly flag), use WithReadOnlyExplicit. If ctx already carries
// an explicit marker, WithReadOnly preserves it.
func WithReadOnly(ctx context.Context) context.Context {
	if IsReadOnlyExplicit(ctx) {
		return ctx
	}
	return context.WithValue(ctx, readOnlyCtxKey{}, false)
}

// WithReadOnlyExplicit is like WithReadOnly, but additionally marks the
// hint as explicit user intent (e.g. sq sql --readonly). Drivers may treat
// an explicit hint more forcefully, e.g. overriding access_mode=AUTOMATIC
// on a DuckDB location.
func WithReadOnlyExplicit(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyCtxKey{}, true)
}

// IsReadOnly reports whether ctx was marked read-only via WithReadOnly or
// WithReadOnlyExplicit.
func IsReadOnly(ctx context.Context) bool {
	_, ok := ctx.Value(readOnlyCtxKey{}).(bool)
	return ok
}

// IsReadOnlyExplicit reports whether ctx was marked read-only via
// WithReadOnlyExplicit, i.e. the read-only request is explicit user intent
// rather than an implicit hint from a non-writing command.
func IsReadOnlyExplicit(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyCtxKey{}).(bool)
	return v
}
