package driver

// AccessMode describes how a source should be opened: read-write (the
// default), or read-only with the intent being either implicit (a
// non-writing command such as sq, inspect, diff, or ping) or explicit
// (the user asked directly, e.g. sq sql --readonly).
//
// AccessMode is a property of an individual open request, not of the
// source itself: the same source may be opened read-only by one command
// and read-write by another within a single process. It is passed
// explicitly as an argument to Grips.Open and Driver.Open, rather than
// carried ambiently on a context, so that the intent is visible at every
// call site.
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

// IsReadOnly reports whether m is one of the read-only modes (implicit
// ModeReadOnly or explicit ModeReadOnlyExplicit). It checks the known
// read-only values explicitly rather than treating "anything but
// ModeReadWrite" as read-only, so an unset or invalid AccessMode is
// treated as read-write, consistent with suffix and String.
func (m AccessMode) IsReadOnly() bool {
	return m == ModeReadOnly || m == ModeReadOnlyExplicit
}

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
