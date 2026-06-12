// Package secret provides pluggable resolution of ${scheme:path} placeholders
// in source.Source.Location values.
//
// A Registry holds Resolver implementations keyed by scheme (e.g. "keyring",
// "env"). Registry.Expand walks a templated Location string, resolves each
// placeholder via the appropriate Resolver, and substitutes the resolved
// values back. URL-encoding of values that land inside URL userinfo is
// handled automatically.
package secret

import (
	"context"
	"errors"
	"sync"
)

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
// Returns ErrUnknownScheme if scheme has no Resolver.
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

	v, err := resolver.Resolve(ctx, path)
	if err != nil {
		return "", err
	}
	r.memo.Store(key, v)
	return v, nil
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
