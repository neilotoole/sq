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
	case ModeReadWrite:
		return "rw"
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
	case ModeReadWrite:
		return "read-write"
	default:
		return "read-write"
	}
}

// readOnlyCtxKey carries an AccessMode on ctx for the paths that still
// rely on ambient propagation. As of Approach 1a, driver.Driver.Open and
// Ping take an explicit mode parameter, so Grips no longer uses ctx to
// reach the driver. This key now serves only the bypass path: code that
// resolves the source and opens a driver directly (currently
// verifySourceCatalogSchema), reached through the shared determineSources
// plumbing that does not yet thread a mode argument. 1b removes this by
// threading mode through that plumbing, after which the key and these
// helpers can be deleted.
type readOnlyCtxKey struct{}

// WithMode stores mode on ctx. Used only by commands that feed the
// verifySourceCatalogSchema bypass (which reads it back via IsReadOnly /
// IsReadOnlyExplicit and then passes the resolved mode explicitly to
// Driver.Open). Not used to reach drivers opened through Grips: those get
// the mode as an explicit Grips.Open / Driver.Open argument.
func WithMode(ctx context.Context, mode AccessMode) context.Context {
	if mode == ModeReadWrite {
		return ctx
	}
	return context.WithValue(ctx, readOnlyCtxKey{}, mode)
}

// IsReadOnly reports whether ctx carries a read-only AccessMode
// (implicit or explicit). Read by the verifySourceCatalogSchema bypass
// to recover a mode set by the command; drivers no longer read ctx (they
// take an explicit mode parameter).
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
