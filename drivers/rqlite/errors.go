package rqlite

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
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

// gorqlitePeersPreamble is the prefix gorqlite puts on the error it
// builds when every peer attempt failed. gorqlite serializes that
// error to a flat string (errors.New on a strings.Builder), so on the
// query path this substring is the only discovery-failure signal that
// survives. gorqlite-specific phrasing; revisit if gorqlite changes
// its error construction (same tradeoff as isTLSSignal's substring
// check).
const gorqlitePeersPreamble = "tried all peers unsuccessfully"

// peerURLPattern extracts candidate peer URLs from gorqlite's
// serialized "tried all peers" error text, which lists each attempt
// in the form:
//
//	peer #0: http://user:xxxxx@rqlite1:4001/db/query?... failed due to ...
var peerURLPattern = regexp.MustCompile(`peer #\d+: (https?://[^\s"]+)`)

// DiscoveryError indicates that gorqlite cluster discovery routed a
// request to an advertised peer that this client cannot use: the
// single-node-localhost trap (a Docker node advertising a
// container-internal hostname) and its variants. It implements
// errz.HumanReadable so the CLI can print a concise, actionable
// message while the full diagnostic chain (the serialized gorqlite
// "tried all peers" detail) stays available via Error and Unwrap for
// logs and verbose output.
type DiscoveryError struct {
	cause error

	// Handle is the source handle, e.g. "@rq".
	Handle string

	// Peer is the advertised peer host that failed, e.g. "rqlite1".
	Peer string

	// UserHost is the host from the source location, e.g. "localhost".
	UserHost string

	// Resolve is true when the peer hostname failed DNS resolution
	// ("no such host"), false when the peer address resolved but could
	// not be reached (dial timeout, connection refused).
	Resolve bool
}

// Error implements error. It carries the full diagnostic form: the
// hint plus the underlying cause chain.
func (e *DiscoveryError) Error() string {
	verb, desc := e.verbDesc()
	msg := fmt.Sprintf(
		"rqlite: cluster discovery failed to %s advertised peer %q "+
			"(not %q from the source URL); this usually means the rqlite "+
			"node advertised %s. Try ?disableClusterDiscovery=true",
		verb, e.Peer, e.UserHost, desc,
	)
	if e.cause == nil {
		return msg
	}
	return msg + ": " + e.cause.Error()
}

// Unwrap returns the underlying cause.
func (e *DiscoveryError) Unwrap() error { return e.cause }

// HumanError implements errz.HumanReadable.
func (e *DiscoveryError) HumanError() string {
	adj := "reachable"
	if e.Resolve {
		adj = "resolvable"
	}
	prefix := ""
	if e.Handle != "" {
		prefix = e.Handle + ": "
	}
	return fmt.Sprintf(
		"%srqlite: cluster discovery failed: advertised peer %q is not %s "+
			"from this host",
		prefix, e.Peer, adj,
	)
}

// verbDesc returns the resolution-vs-reachability wording pair.
func (e *DiscoveryError) verbDesc() (verb, desc string) {
	if e.Resolve {
		return "resolve", "an internal hostname not resolvable from the host"
	}
	return "reach", "an internal address not reachable from this host"
}

// rewritePeerDiscoveryError rewrites a gorqlite cluster discovery
// failure into a *DiscoveryError naming the problematic advertised
// peer and pointing at ?disableClusterDiscovery=true.
// Pass-through in every other case (nil err, unrelated err, the
// failing host matches the host the user typed, discovery already
// disabled, src.Location unparseable).
//
// Two detection paths:
//
//  1. Chain-preserving: err wraps a *net.DNSError with IsNotFound set.
//     This only fires for errors that preserve the error chain (e.g.
//     sq-side code wrapping a transport error); gorqlite's query path
//     never produces these.
//  2. Text: gorqlite's rqliteApiCall serializes transport errors to a
//     flat string, so on the query path the message text is the only
//     available signal. When the text carries gorqlite's "tried all
//     peers" preamble, the peer URLs are parsed out and compared
//     against the host from src.Location. A peer host that differs
//     from the user's host is the discovery trap: the node advertised
//     an address (internal hostname, container IP) that this client
//     cannot use. This catches both unresolvable hostnames ("no such
//     host") and resolvable-but-unreachable addresses (dial timeouts,
//     connection refused).
//
// The chain-preserving rewrite keeps the underlying *net.DNSError
// reachable so upstream errors.As classification continues to work.
func rewritePeerDiscoveryError(err error, src *source.Source) error {
	if err == nil {
		return nil
	}
	u, parseErr := url.Parse(src.Location)
	if parseErr != nil {
		// Defensive: doOpen validates this URL earlier, but if we
		// somehow can't parse it here, pass the error through rather
		// than producing a misleading rewrite.
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
	userHost := u.Hostname()

	// Path 1: chain-preserved DNS not-found. Timeouts, temporary
	// failures, and refusals are unrelated classes and shouldn't be
	// rewritten into a disableClusterDiscovery suggestion.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		if strings.EqualFold(dnsErr.Name, userHost) {
			// The failing hostname is the one the user typed. That's
			// their problem to fix; suggesting disableClusterDiscovery
			// would be wrong.
			return err
		}
		return errz.Err(&DiscoveryError{
			cause:    err,
			Handle:   src.Handle,
			Peer:     dnsErr.Name,
			UserHost: userHost,
			Resolve:  true,
		})
	}

	// Path 2: gorqlite string-serialized "tried all peers" failure.
	// firstForeignPeer applies the connection-level classification
	// per peer segment: only a foreign peer whose own failure is
	// DNS lookup or dial level indicates the discovery trap.
	text := err.Error()
	if !strings.Contains(text, gorqlitePeersPreamble) {
		return err
	}
	peerHost, resolve := firstForeignPeer(text, userHost)
	if peerHost == "" {
		// Every parseable peer is the host the user typed (or none
		// parsed): not the discovery trap.
		return err
	}
	return errz.Err(&DiscoveryError{
		cause:    err,
		Handle:   src.Handle,
		Peer:     peerHost,
		UserHost: userHost,
		Resolve:  resolve,
	})
}

// firstForeignPeer parses the peer entries out of gorqlite's
// serialized "tried all peers" error text and returns the host of the
// first peer that exhibits the discovery trap: a foreign host (differs
// from userHost) whose own failure segment is connection-level. The
// returned resolve is true for a DNS not-found, false for a dial
// failure. All classification is scoped to each peer's own text
// segment: with multiple peers failing differently, a whole-text check
// would misattribute one peer's failure class to another.
//
// Skipped peers, in addition to unparseable ones:
//
//   - Same host as userHost. Loopback hosts count as equal
//     ("localhost" vs "127.0.0.1" vs "::1"): a node legitimately
//     advertising loopback to a loopback-typed source is not the
//     trap, even when the strings differ.
//   - Canceled attempts ("operation was canceled", "context
//     canceled"): the user or a context aborted mid-dial, which says
//     nothing about the peer being unusable. Deadline expiries remain
//     eligible: an unroutable advertised peer typically surfaces as a
//     dial timeout.
//   - HTTP-level failures (e.g. a 401): the peer was reached; that's
//     a different problem, not the discovery trap.
//
// Marker matching is case-insensitive so platform variants match
// (Windows: "No such host is known").
func firstForeignPeer(text, userHost string) (host string, resolve bool) {
	userLoopback := isLoopbackHost(userHost)
	matches := peerURLPattern.FindAllStringSubmatchIndex(text, -1)
	for i, m := range matches {
		pu, err := url.Parse(text[m[2]:m[3]])
		if err != nil {
			continue
		}
		h := pu.Hostname()
		if h == "" || strings.EqualFold(h, userHost) {
			continue
		}
		if userLoopback && isLoopbackHost(h) {
			continue
		}
		// This peer's failure detail runs from its entry to the next
		// peer entry (or the end of the text).
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		segment := strings.ToLower(text[m[0]:end])
		switch {
		case strings.Contains(segment, "operation was canceled"),
			strings.Contains(segment, "context canceled"):
			continue
		case strings.Contains(segment, "no such host"):
			return h, true
		case strings.Contains(segment, "dial tcp"):
			return h, false
		default:
			continue
		}
	}
	return "", false
}

// AuthError indicates that the rqlite node rejected a request as
// unauthorized (HTTP 401): the node requires credentials that the
// source either doesn't carry or that the node doesn't accept. It
// implements errz.HumanReadable so the CLI can print a concise,
// actionable message while the full diagnostic chain stays available
// via Error and Unwrap for logs and verbose output.
type AuthError struct {
	cause error

	// Handle is the source handle, e.g. "@rq".
	Handle string

	// HasCreds is true when the source location carries userinfo
	// (username and/or password). It selects between the "supply
	// credentials" and "check credentials" remedies.
	HasCreds bool
}

// Error implements error. It carries the full diagnostic form: the
// hint plus the underlying cause chain.
func (e *AuthError) Error() string {
	msg := "rqlite: auth failed (401 Unauthorized)"
	if e.cause == nil {
		return msg
	}
	return msg + ": " + e.cause.Error()
}

// Unwrap returns the underlying cause.
func (e *AuthError) Unwrap() error { return e.cause }

// HumanError implements errz.HumanReadable.
func (e *AuthError) HumanError() string {
	prefix := ""
	if e.Handle != "" {
		prefix = e.Handle + ": "
	}
	if e.HasCreds {
		return prefix + "rqlite: auth failed: node rejected credentials"
	}
	return prefix + "rqlite: auth failed: node requires credentials, " +
		"but source has none"
}

// rewriteAuthError rewrites a 401 Unauthorized failure from the
// rqlite node into a *AuthError. Detection is text-based, like the
// other gorqlite-path checks, and anchored to gorqlite's exact
// serialization: its doOnce is the module's only HTTP call site, and
// it formats every non-200 as "got: <status>, message: <body>", so
// "got: 401 Unauthorized" covers every gorqlite 401 path while not
// matching quoted content (e.g. a non-401 response body that merely
// mentions a 401). Pass-through for nil and non-401 errors.
func rewriteAuthError(err error, src *source.Source) error {
	if err == nil || !strings.Contains(err.Error(), "got: 401 Unauthorized") {
		return err
	}
	// Conservative default: if the location can't be parsed (e.g. it
	// carries secret placeholders in userinfo, which net/url rejects),
	// userinfo is present, so treat creds as supplied.
	hasCreds := true
	if u, parseErr := url.Parse(src.Location); parseErr == nil {
		hasCreds = u.User != nil && u.User.String() != ""
	}
	return errz.Err(&AuthError{
		cause:    err,
		Handle:   src.Handle,
		HasCreds: hasCreds,
	})
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
	// If the user already set ?tls=true, the TLS signal is not a
	// wrong-scheme hint; surfacing "retry with ?tls=true" would
	// mislead them. Let the raw error through.
	if locHasTLSTrue(src.Location) {
		return err
	}
	hint := suggestLocWithParams(src, url.Values{"tls": {"true"}})
	return errz.WithHuman(
		errz.Wrapf(err,
			"%s appears to require TLS; retry with %s "+
				"(add &insecure=true for self-signed certs)",
			src.Handle, hint),
		humanMsg(src, "rqlite: TLS required: endpoint serves HTTPS, but source uses HTTP"),
	)
}

// humanMsg prefixes msg with the source handle, yielding the standard
// human-message shape: "@handle: driver: category failed: detail".
func humanMsg(src *source.Source, msg string) string {
	if src == nil || src.Handle == "" {
		return msg
	}
	return src.Handle + ": " + msg
}

// rewritePlainHTTPSignalError is the inverse of rewriteTLSSignalError:
// if err looks like an HTTPS request answered by a plain-HTTP server,
// and the source has ?tls=true set, the source's TLS setting is the
// mismatch. Pass-through in every other case. Like the other
// gorqlite-path checks, detection is text-based: Go's http transport
// emits the canonical "server gave HTTP response to HTTPS client"
// message, which survives gorqlite's string serialization.
func rewritePlainHTTPSignalError(err error, src *source.Source) error {
	if err == nil ||
		!strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
		return err
	}
	// Only rewrite when the source actually opts into TLS: that's the
	// setting the message blames. (Without tls=true, sq doesn't speak
	// HTTPS on this path, so the signal would be something else.)
	if !locHasTLSTrue(src.Location) {
		return err
	}
	// No URL hint in the long form: the remedy is removing params
	// (tls=true, and insecure=true if present), which
	// suggestLocWithParams cannot express.
	return errz.WithHuman(
		errz.Wrapf(err,
			"%s: tls=true is set, but the endpoint serves plain HTTP; "+
				"remove tls=true (and insecure=true, if set) from the "+
				"source location", src.Handle),
		humanMsg(src, "rqlite: TLS mismatch: endpoint serves HTTP, but source uses HTTPS"),
	)
}

// locHasTLSTrue reports whether loc has ?tls=true set. Used to gate
// TLS-signal error enrichment: if the user has already opted into
// TLS, an io.EOF or TLS handshake failure is NOT a "wrong scheme"
// indication and the suggestion to add ?tls=true would be misleading.
func locHasTLSTrue(loc string) bool {
	u, err := url.Parse(loc)
	if err != nil {
		return false
	}
	return u.Query().Get("tls") == "true"
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
	return errz.WithHuman(
		errz.Wrapf(err,
			"%s: TLS certificate verification failed. If this is a "+
				"self-signed or private-CA deployment, retry with "+
				"%s, or install the CA in your trust store",
			src.Handle, hint),
		humanMsg(src, "rqlite: TLS cert verification failed"),
	)
}

// enrichConnError applies the known connection-error enrichments,
// returning the result of the first rewrite that matches.
// First-match-wins matters: a serialized gorqlite "tried all peers"
// failure can carry multiple signals at once (e.g. a dial failure on
// one peer and a 401 from another), and because each wrapper's
// Error() embeds the cause text, a later check would re-match on the
// already-wrapped error and bury the first diagnosis under a second
// wrapper. Order: peer discovery first (most specific), then auth
// (401), then the two TLS-mismatch signals (HTTP→HTTPS and
// HTTPS→HTTP, mutually exclusive via the tls=true gate), then cert
// verification (HTTPS with bad cert).
func enrichConnError(err error, src *source.Source) error {
	if err == nil {
		return nil
	}
	for _, rewrite := range []func(error, *source.Source) error{
		rewritePeerDiscoveryError,
		rewriteAuthError,
		rewriteTLSSignalError,
		rewritePlainHTTPSignalError,
		rewriteCertVerificationError,
	} {
		// Identity comparison, not errors.Is: every rewrite returns
		// the input unchanged when it doesn't match.
		if out := rewrite(err, src); out != err { //nolint:errorlint
			return out
		}
	}
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
