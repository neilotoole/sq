// Package secret provides pluggable resolution of ${scheme:path} placeholders
// in source.Source.Location values.
//
// A Registry holds Resolver implementations keyed by scheme (e.g. "keyring",
// "env"). Registry.Expand walks a templated Location string, resolves each
// placeholder via the appropriate Resolver, and substitutes the resolved
// values back. URL-encoding of values that land inside URL userinfo is
// handled automatically.
//
// # Grammar
//
// A placeholder is ${scheme:path}, where scheme matches [a-z][a-z0-9]*
// and path is any non-empty text up to the first '}'. In literal text,
// "$$" escapes a literal '$'. Because the path ends at the first '}', a
// path containing '}' cannot be expressed. To prevent such a path from
// silently truncating, a '}' in literal text is constrained: before the
// first placeholder it is always literal; immediately after a
// placeholder it is literal (so "${env:X}}" is the placeholder followed
// by '}'); anywhere else after a placeholder it must be balanced by an
// earlier unmatched literal '{', or parsing fails with an unbalanced-'}'
// error.
//
// # Templates vs literals
//
// Two kinds of strings flow through this package, and confusing them
// silently corrupts credentials:
//
//   - A template contains ${scheme:path} refs and uses "$$" to escape a
//     literal '$'. Stored source locations (sq.yml, config exports) are
//     templates.
//   - A literal is raw bytes. Resolver outputs, keyring slot values, and
//     expanded locations (e.g. from driver.ResolveSourceSecrets) are
//     literals; they are spliced or used verbatim, never re-scanned.
//
// Every boundary that moves bytes between the two kinds must convert
// exactly once: Escape converts literal -> template; Expand and Unescape
// convert template -> literal (Unescape only for templates with no refs;
// use Registry.Expand when refs may be present). Storing template bytes
// in a literal slot (or vice versa), or converting twice, mangles any
// '$' the value contains. source.Source.SecretsResolved marks an
// in-memory source whose Location has already been converted to literal
// form.
package secret

import (
	"context"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// resolveTimeout is a backstop upper bound on a single detached secret
// resolution flight (see Registry.ResolveScheme). It is intentionally
// generous: it must accommodate an interactive resolver (e.g. a 1Password
// "op read" awaiting biometric auth), and exists only to prevent a wholly
// stuck resolution from running indefinitely, not as the normal timeout.
const resolveTimeout = 2 * time.Minute

// ErrNotFound is returned by Resolver implementations when a secret does
// not exist. Callers may use errors.Is to detect this case.
var ErrNotFound = errors.New("secret not found")

// ErrUnknownScheme is returned when an Expand request references a scheme
// that has no registered Resolver.
var ErrUnknownScheme = errors.New("unknown secret scheme")

// Resolver returns secret values for placeholder paths.
type Resolver interface {
	// Resolve returns the secret value for the path part of a placeholder
	// (e.g. "my_db_pw" for ${keyring:my_db_pw}). The scheme
	// has already been dispatched to this Resolver by the Registry.
	Resolve(ctx context.Context, path string) (string, error)
}

// Registry maps schemes to Resolvers.
type Registry struct {
	resolvers map[string]Resolver

	// flight coalesces concurrent resolutions of the same scheme:path
	// into a single backend hit, mirroring the op resolver. Backends can
	// be expensive or interactive (keyring does OS-keychain IPC that may
	// prompt; op may trigger a biometric prompt), so concurrent callers
	// must never fan out into duplicate prompts.
	flight singleflight.Group

	// memo caches successful resolutions, keyed by scheme and path, for
	// the Registry's lifetime, which is a single CLI invocation. Several
	// code paths resolve the same location independently (e.g.
	// --src.schema validation opens a probe connection, then Grips.doOpen
	// opens the real one); without the memo each pass costs a fresh
	// backend hit, which for keyring is an OS keychain roundtrip that may
	// prompt the user. The memo also gives one invocation a consistent
	// view of each secret. Failures are not cached, so transient backend
	// errors don't stick.
	memo sync.Map

	mu sync.RWMutex
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{resolvers: make(map[string]Resolver)}
}

// Register associates resolver with scheme. Subsequent calls with the same
// scheme overwrite the prior registration.
func (r *Registry) Register(scheme string, resolver Resolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolvers[scheme] = resolver
}

// ResolveScheme dispatches a single scheme/path pair to the appropriate
// Resolver, memoizing successful resolutions for the Registry's lifetime.
// Concurrent calls for the same scheme:path coalesce into one backend
// hit; a failure in that shared flight is replayed to every waiter but
// not cached, so a sequential retry reaches the backend again. Returns
// ErrUnknownScheme if scheme has no Resolver.
func (r *Registry) ResolveScheme(ctx context.Context, scheme, path string) (string, error) {
	// NUL can't occur in a scheme (validateScheme permits only
	// [a-z0-9]), so the key is unambiguous.
	key := scheme + "\x00" + path
	if v, ok := r.memo.Load(key); ok {
		return v.(string), nil
	}

	r.mu.RLock()
	resolver, ok := r.resolvers[scheme]
	r.mu.RUnlock()
	if !ok {
		return "", ErrUnknownScheme
	}

	// DoChan (not Do) so each caller can honor its own ctx while waiting:
	// a caller can abort independently (the select below) without
	// affecting the in-flight resolution.
	//
	// The shared resolution runs on a context detached from any single
	// caller's cancellation. The closure runs under the first caller's
	// goroutine, so binding it to that caller's ctx would let the leader's
	// cancellation fail the flight and replay a spurious "context canceled"
	// to every other (healthy) waiter. The shared work belongs to the
	// flight, not the leader, so we drop the leader's deadline/cancel
	// (context.WithoutCancel keeps ctx values such as the registry).
	//
	// The resolvers do not self-bound: keyring/env/file ignore ctx, and op
	// shells out via exec.CommandContext with no internal timeout. So we
	// give the detached flight a generous timeout of our own; otherwise a
	// stuck resolution (e.g. an op subprocess awaiting a biometric prompt
	// the user never answers) could run for the life of the process. The
	// caller's own ctx (the select below) and OS signals remain the normal
	// cancellation paths; this is only a backstop.
	flightCtx, cancelFlight := context.WithTimeout(context.WithoutCancel(ctx), resolveTimeout)
	ch := r.flight.DoChan(key, func() (any, error) {
		defer cancelFlight()
		// Re-check the memo: a concurrent flight may have populated it
		// while this caller was waiting on the singleflight lock.
		if v, ok := r.memo.Load(key); ok {
			return v.(string), nil
		}
		v, err := resolver.Resolve(flightCtx, path)
		if err != nil {
			return nil, err
		}
		r.memo.Store(key, v)
		return v, nil
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		return res.Val.(string), nil
	case <-ctx.Done():
		return "", errz.Err(ctx.Err())
	}
}

type ctxKey struct{}

// NewContext returns a context carrying reg.
func NewContext(parent context.Context, reg *Registry) context.Context {
	return context.WithValue(parent, ctxKey{}, reg)
}

// FromContext returns the Registry carried by ctx, or nil if none.
func FromContext(ctx context.Context) *Registry {
	r, _ := ctx.Value(ctxKey{}).(*Registry)
	return r
}
