package driver

import "context"

// readOnlyCtxKey is the (unexported) context key carrying the read-only hint.
type readOnlyCtxKey struct{}

// WithReadOnly returns a derived context that signals to driver.Driver
// implementations that the caller does not intend to write to the source.
// Drivers that can honor this hint (currently: DuckDB) should connect in
// read-only mode; drivers that cannot must ignore it.
//
// The hint is advisory. A driver that ignores it MUST still produce a
// working connection.
func WithReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyCtxKey{}, true)
}

// IsReadOnly reports whether ctx was marked read-only via WithReadOnly.
func IsReadOnly(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyCtxKey{}).(bool)
	return v
}
