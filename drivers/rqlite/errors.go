package rqlite

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// errw wraps any error from the db. It should be called at every
// interaction with the db. If err is nil, nil is returned. Certain errors
// are wrapped in specific error types, e.g. errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "no such table:"):
		// rqlite returns SQLite-formatted error messages over the wire,
		// so the "no such table:" prefix carries through verbatim.
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

// docsLocalhostAnchor is the docs URL for the single-node-localhost
// case. Kept as a package-level constant so the add-time hint and the
// DNS error rewrite point at the same place.
const docsLocalhostAnchor = "https://sq.io/docs/drivers/rqlite#single-node-localhost"

// maybeWarnLocalhostDiscovery emits a one-line Warn log when src's URL
// host is loopback AND disableClusterDiscovery is not explicitly set on
// the query string. Single-node localhost (Docker container reached
// from the host) is the most common newcomer setup and gorqlite's
// default cluster discovery fails opaquely there; a Warn at add/open
// time provides a breadcrumb in the log file pointing at the docs.
//
// Best-effort: any failure to parse src.Location or extract the host
// is a silent no-op. The warning must never affect Open behavior.
func maybeWarnLocalhostDiscovery(ctx context.Context, src *source.Source) {
	u, err := url.Parse(src.Location)
	if err != nil {
		return
	}
	v := u.Query().Get("disableClusterDiscovery")
	if strings.EqualFold(v, "true") || strings.EqualFold(v, "false") {
		// User has made an explicit choice (true or false). Don't
		// second-guess them. Empty or unrecognized values fall through
		// to the warning so a typo like ?disableClusterDiscovery=yes
		// still gets surfaced.
		return
	}
	host := u.Hostname()
	if host == "" {
		return
	}
	if !isLoopbackHost(host) {
		return
	}
	lg.FromContext(ctx).Warn(
		"rqlite: source points at loopback but disableClusterDiscovery is not set; "+
			"single-node localhost setups typically need ?disableClusterDiscovery=true "+
			"to avoid peer-discovery failures from the host. See "+docsLocalhostAnchor,
		lga.Src, src.Handle,
	)
}

// isLoopbackHost reports whether host is a literal loopback reference.
// Matches "localhost" (case-insensitive) and any IP whose net.IP
// representation reports IsLoopback (covers 127.0.0.0/8, ::1, and
// ::ffff:127.x.x.x mappings). No DNS resolution is performed.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// rewritePeerDNSError rewrites a gorqlite cluster-discovery DNS
// failure into an actionable message naming the unreachable peer and
// pointing at ?disableClusterDiscovery=true and the docs. Pass-through
// in every other case (nil err, non-DNS err, user-supplied host
// matches the failing name, discovery already disabled, src.Location
// unparseable).
//
// The rewrite preserves the underlying *net.DNSError so upstream
// errors.As classification continues to work.
func rewritePeerDNSError(err error, src *source.Source) error {
	if err == nil {
		return nil
	}
	var dnsErr *net.DNSError
	if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
		// Only the "no such host" case (IsNotFound) is the
		// cluster-discovery DNS failure we want to rewrite. DNS
		// timeouts, temporary failures, and refusals are unrelated
		// classes and shouldn't be rewritten into a
		// disableClusterDiscovery suggestion.
		return err
	}
	u, parseErr := url.Parse(src.Location)
	if parseErr != nil {
		// Defensive: doOpen validates this URL earlier, but if we
		// somehow can't parse it here, pass the error through rather
		// than producing a misleading rewrite.
		return err
	}
	userHost := u.Hostname()
	if strings.EqualFold(dnsErr.Name, userHost) {
		// The failing hostname is the one the user typed. That's
		// their problem to fix; suggesting disableClusterDiscovery
		// would be wrong.
		return err
	}
	if strings.EqualFold(u.Query().Get("disableClusterDiscovery"), "true") {
		// Discovery already off; failure is something else.
		// gorqlite's underlying parser treats "true"/"TRUE"/"True"
		// equivalently, so match its case-insensitive interpretation
		// rather than producing a misleading rewrite that suggests the
		// user "try ?disableClusterDiscovery=true" when they already have.
		return err
	}
	return errz.Wrapf(err,
		"rqlite: cluster-discovery failed to resolve advertised peer %q "+
			"(not %q from the source URL); this usually means the rqlite "+
			"node advertised an internal hostname not resolvable from the "+
			"host. Try ?disableClusterDiscovery=true, or see "+docsLocalhostAnchor,
		dnsErr.Name, userHost,
	)
}

// suggestLocWithParams builds a hint URL by merging the given query
// params into src.RedactedLocation(). Use this for error-enrichment
// hints that show the user "retry with this URL." Existing query
// params survive; the new params override any existing key.
//
// Falls back to a best-effort string concat if the redacted location
// can't be parsed (which should never happen in practice, since
// RedactedLocation produces a valid URL).
func suggestLocWithParams(src *source.Source, params url.Values) string {
	loc := src.RedactedLocation()
	u, err := url.Parse(loc)
	if err != nil {
		// Defensive fallback.
		sep := "?"
		if strings.Contains(loc, "?") {
			sep = "&"
		}
		return loc + sep + params.Encode()
	}
	q := u.Query()
	for k, vs := range params {
		for _, v := range vs {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// rewriteTLSSignalError, if err looks like the server wanted TLS but
// the client spoke plain HTTP, returns a new error suggesting
// ?tls=true. Otherwise returns err unchanged.
//
// We deliberately do not auto-retry over HTTPS at this stage. An
// earlier design considered a probe (gh756 option B / option A+B
// hybrid) but concluded that a one-off Prober interface is not
// justified for a single driver. See gh764 for the deferred
// follow-up; the natural home for any future TLS-vs-plaintext
// auto-detection is the AddHinter mechanism (gh755), not a
// rqlite-specific probe.
func rewriteTLSSignalError(err error, src *source.Source) error {
	if err == nil || !isTLSSignal(err) {
		return err
	}
	hint := suggestLocWithParams(src, url.Values{"tls": {"true"}})
	return errz.Wrapf(err,
		"%s appears to require TLS; retry with %s "+
			"(add &insecure=true for self-signed certs)",
		src.Handle, hint)
}

// isTLSSignal reports whether err looks like the failure of a plain
// HTTP request against a TLS-only server. The three checks are
// conservative: false negatives just produce the unwrapped error,
// which is still actionable for the user.
//
// In production today, only the substring check (3) fires. gorqlite's
// rqliteApiCall serializes all transport errors to strings via
// errors.New(builder.String()) before returning, breaking the error
// chain — so errors.As / errors.Is on transport-layer types (1) and
// (2) cannot match. Those checks are retained as forward-compat
// defenses in case gorqlite later preserves the chain, or in case a
// caller passes a non-gorqlite error (e.g. wrapped from a custom
// probe).
func isTLSSignal(err error) bool {
	if err == nil {
		return false
	}

	// 1. Go's net/http detected a TLS record on a plain-HTTP socket.
	// Dead in production today (gorqlite breaks the error chain) but
	// retained for forward-compat.
	var rec tls.RecordHeaderError
	if errors.As(err, &rec) {
		return true
	}

	// 2. Server closed mid-handshake (a common TLS-only response
	// to an HTTP request). Dead in production today (gorqlite breaks
	// the error chain) but retained for forward-compat.
	if errors.Is(err, io.EOF) {
		return true
	}

	// 3. gorqlite-wrapped error containing the canonical 400 body
	// Go's net/http server emits when reached over plain HTTP on a
	// TLS listener. This is the only check that fires in production.
	// gorqlite/Go-net-http-specific; revisit if rqlite changes its
	// HTTP stack.
	if strings.Contains(err.Error(), "HTTP request to an HTTPS server") {
		return true
	}
	return false
}

// rewriteCertVerificationError, if err looks like a TLS certificate
// verification failure, wraps it with a hint pointing at
// ?insecure=true (for self-signed deployments) and installing the
// CA (for production). Otherwise returns err unchanged.
func rewriteCertVerificationError(err error, src *source.Source) error {
	if err == nil || !isCertVerificationError(err) {
		return err
	}
	hint := suggestLocWithParams(src, url.Values{"tls": {"true"}, "insecure": {"true"}})
	return errz.Wrapf(err,
		"%s: TLS certificate verification failed. If this is a "+
			"self-signed or private-CA deployment, retry with "+
			"%s, or install the CA in your trust store",
		src.Handle, hint)
}

// enrichConnError applies the known connection-error enrichments
// in a fixed order. Each inner check returns the input unchanged
// if it doesn't match, so the composition is safe and idempotent.
// Order matters only for readability: DNS first (most specific),
// then TLS signal (HTTP→HTTPS), then cert verification (HTTPS
// with bad cert).
func enrichConnError(err error, src *source.Source) error {
	err = rewritePeerDNSError(err, src)
	err = rewriteTLSSignalError(err, src)
	err = rewriteCertVerificationError(err, src)
	return err
}

// isCertVerificationError reports whether err is (or wraps) one of
// the x509 / crypto/tls verification error types. Used by the
// enrichment to decide whether to suggest ?insecure=true.
//
// In production today, only the substring check (the last branch)
// reliably fires, because gorqlite's rqliteApiCall serializes
// transport errors via errors.New(builder.String()), breaking the
// errors.As chain. The errors.As checks are retained as forward-
// compat defenses for a future gorqlite that preserves the chain,
// or callers that pass a non-gorqlite error.
func isCertVerificationError(err error) bool {
	if err == nil {
		return false
	}
	var unkAuth x509.UnknownAuthorityError
	if errors.As(err, &unkAuth) {
		return true
	}
	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return true
	}
	var verifyErr *tls.CertificateVerificationError
	if errors.As(err, &verifyErr) {
		return true
	}
	// Substring fallback: the canonical "x509:" prefix on Go's
	// certificate verification errors survives gorqlite's string
	// serialization, so the substring check is the workhorse in
	// production. Match conservatively on "x509:" since that prefix
	// is reserved for x509 errors in Go's stdlib.
	if strings.Contains(err.Error(), "x509:") {
		return true
	}
	return false
}
