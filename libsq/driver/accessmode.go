package driver

import "context"

// AccessMode describes how a source should be opened: read-write (the
// default), or read-only with the hint being either implicit (a
// non-writing command such as sq, inspect, diff, ping, or add-time
// validation) or explicit (the user asked directly, e.g. sq sql
// --readonly).
//
// AccessMode is a property of an individual open request, not of the
// source itself: the same source may be opened read-only by one command
// and read-write by another within a single process. It is passed
// explicitly to Grips.Open via OpenOpt values, rather than carried
// ambiently on a context, so that the intent is visible at every call
// site.
type AccessMode uint8

const (
	// ModeReadWrite is the default: the caller may write to the source.
	ModeReadWrite AccessMode = iota

	// ModeReadOnly is an implicit read-only hint from a non-writing code
	// path. It is advisory: a driver that cannot honor it (anything but
	// DuckDB today) must still return a working connection, and it never
	// overrides an access mode the user pinned on the source location.
	ModeReadOnly

	// ModeReadOnlyExplicit is explicit user intent (sq sql --readonly).
	// Drivers may treat it more forcefully than ModeReadOnly, e.g.
	// DuckDB overrides access_mode=AUTOMATIC on a file location only for
	// an explicit hint.
	ModeReadOnlyExplicit
)

// suffix returns the cache-key suffix for the mode. Stable and distinct
// per mode so that a source opened in two modes within one run gets two
// coexisting grips (gh #779).
func (m AccessMode) suffix() string {
	switch m {
	case ModeReadOnlyExplicit:
		return "rox"
	case ModeReadOnly:
		return "ro"
	default:
		return "rw"
	}
}

// String implements fmt.Stringer.
func (m AccessMode) String() string {
	switch m {
	case ModeReadOnlyExplicit:
		return "read-only (explicit)"
	case ModeReadOnly:
		return "read-only"
	default:
		return "read-write"
	}
}

// OpenOpt configures a Grips.Open call. The zero set of opts yields
// ModeReadWrite.
type OpenOpt func(*AccessMode)

// ReadOnly requests an implicit read-only open. It does not downgrade an
// already-explicit request, so combining ReadOnly with ReadOnlyExplicit
// in either order yields ModeReadOnlyExplicit.
func ReadOnly() OpenOpt {
	return func(m *AccessMode) {
		if *m < ModeReadOnly {
			*m = ModeReadOnly
		}
	}
}

// ReadOnlyExplicit requests an explicit read-only open (e.g. the user
// passed sq sql --readonly).
func ReadOnlyExplicit() OpenOpt {
	return func(m *AccessMode) {
		*m = ModeReadOnlyExplicit
	}
}

// Mode returns an OpenOpt that sets the access mode directly.
// Convenience for callers that carry an AccessMode value (e.g.
// QueryContext) rather than composing ReadOnly/ReadOnlyExplicit.
func Mode(mode AccessMode) OpenOpt {
	return func(m *AccessMode) {
		if *m < mode {
			*m = mode
		}
	}
}

// resolveMode applies opts to the default ModeReadWrite.
func resolveMode(opts []OpenOpt) AccessMode {
	mode := ModeReadWrite
	for _, opt := range opts {
		if opt != nil {
			opt(&mode)
		}
	}
	return mode
}

// readOnlyCtxKey carries the resolved AccessMode from Grips.Open down to
// driver.Driver.Open. This is an internal protocol between Grips and the
// drivers: Grips sets it just before calling Driver.Open, and a driver
// reads it via IsReadOnly / IsReadOnlyExplicit. Application code no
// longer sets this; it passes OpenOpt values to Grips.Open instead.
type readOnlyCtxKey struct{}

// WithMode stores mode on ctx for the Driver.Open hop. Grips calls this
// internally to bridge the explicit OpenOpt API to the driver's
// ctx-based read side (avoiding a signature change to Driver.Open across
// every driver). It is also exported for the rare caller that opens a
// driver directly, bypassing Grips (e.g. verifySourceCatalogSchema's
// uncached validation open): such callers have no OpenOpt seam, so they
// set the mode on ctx themselves. Grips callers should pass OpenOpt
// values to Grips.Open instead of using this.
func WithMode(ctx context.Context, mode AccessMode) context.Context {
	if mode == ModeReadWrite {
		return ctx
	}
	return context.WithValue(ctx, readOnlyCtxKey{}, mode)
}

// IsReadOnly reports whether the source is being opened read-only
// (implicit or explicit). Drivers that can honor the hint (DuckDB) call
// this from their Open method.
func IsReadOnly(ctx context.Context) bool {
	m, ok := ctx.Value(readOnlyCtxKey{}).(AccessMode)
	return ok && m != ModeReadWrite
}

// IsReadOnlyExplicit reports whether the read-only request is explicit
// user intent rather than an implicit hint from a non-writing command.
func IsReadOnlyExplicit(ctx context.Context) bool {
	m, ok := ctx.Value(readOnlyCtxKey{}).(AccessMode)
	return ok && m == ModeReadOnlyExplicit
}
