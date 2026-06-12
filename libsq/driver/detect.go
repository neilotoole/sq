package driver

import (
	"context"
	"net/url"

	"github.com/neilotoole/sq/libsq/source"
)

// ConnParamDetector is an optional interface that drivers can
// implement to detect omitted connection parameters by inspecting
// the live endpoint that a source location points at. The canonical
// example is the rqlite driver detecting that an endpoint requires
// TLS and returning {"tls": ["true"]}, sparing the user a failed
// add and a manual retry with the corrected location.
//
// sq exercises this interface only when a source location is being
// established. Today that means "sq add": after the source has been
// validated, before it is persisted, and never when --skip-verify
// is set. It is not invoked at Open, Ping, or query time, and a
// persisted source is never re-detected; if an endpoint's transport
// changes later, the user sees a connection error with a hint, not
// a silent config rewrite. Implementations must not depend on this
// schedule: it is caller policy, and detection must be safe to
// invoke at any lifecycle point, which follows from the requirement
// that implementations mutate neither src nor any other state.
//
// Detection inspects the live endpoint and therefore touches the
// network. That is definitional, not incidental: it is why
// --skip-verify suppresses detection. A parameter that can be
// derived offline from the location string alone is not detection
// but canonicalization (the family of the drivers' MungeLocation
// functions), and must not be implemented via this interface,
// because canonicalization must not be suppressed by
// --skip-verify. Implementations must honor ctx cancellation and
// should bound each network attempt, e.g. via OptConnOpenTimeout or
// a driver-native timeout param when one exists (the rqlite driver
// gives its ?timeout=N param precedence over OptConnOpenTimeout,
// matching its connection-time behavior).
type ConnParamDetector interface {
	// DetectConnParams inspects the endpoint that src.Location
	// points at, and returns connection query parameters to be
	// merged into the location before it is persisted. src arrives
	// with secret placeholders already resolved. The caller owns
	// the merge onto the stored (possibly placeholder-bearing)
	// form, and the caller decides disposition: implementations
	// MUST NOT assume that returned params will be applied.
	//
	// The returned keys must be a subset of the keys advertised by
	// SQLDriver.ConnParams. Values must be high-confidence
	// observations of the endpoint, never guesses: when in doubt,
	// return nothing.
	//
	// A (nil, nil) return means "nothing detected, or nothing to
	// do", and is the expected result in most situations,
	// including:
	//
	//   - The location already expresses explicit intent (e.g. the
	//     user typed ?tls=true or ?tls=false): detection must not
	//     second-guess the user.
	//   - The endpoint is unreachable or the response is
	//     ambiguous: connection failures are not this method's
	//     concern, and surface with full context from the Ping
	//     that follows during "sq add".
	//
	// A non-nil error aborts the add. It is reserved for the case
	// where detection has learned something that dooms the add AND
	// can be reported more usefully here than by the subsequent
	// Ping. For example: the endpoint demands TLS but presents an
	// unverifiable certificate, where the actionable error names
	// the ?insecure=true escape hatch and the install-the-CA
	// alternative in one message. Errors must be actionable and
	// must not leak credentials from src.Location.
	DetectConnParams(ctx context.Context, src *source.Source) (url.Values, error)
}
