// Package keyring is the OS-keyring backend for libsq/core/secret.
// It wraps github.com/zalando/go-keyring, which supports macOS Keychain,
// Windows Credential Manager, and freedesktop Secret Service on Linux.
package keyring

import (
	"context"
	"errors"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// Service is the fixed keyring service name used for all sq secrets.
const Service = "sq"

// Store is a client for the OS keyring, scoped to the sq service. It
// supports the full read/write/delete lifecycle plus opaque-ID minting
// (NewID, in id.go). Store also satisfies secret.Resolver, so it can
// be registered under the "keyring" scheme on a secret.Registry.
type Store struct{}

// NewStore returns a Store. Callers register the result with a
// secret.Registry under the "keyring" scheme.
func NewStore() *Store {
	return &Store{}
}

// Resolve returns the secret stored at path. Returns secret.ErrNotFound
// when no entry exists.
//
// Each call is an OS-keychain IPC roundtrip. Store deliberately has no
// cache of its own (unlike the op resolver): per-run memoization and
// concurrent-call coalescing happen in secret.Registry, above this
// layer, and the keyring write commands (create, update, rm, migrate)
// use Store directly and must always read through to the backend.
func (s *Store) Resolve(_ context.Context, path string) (string, error) {
	v, err := gokeyring.Get(Service, path)
	if errors.Is(err, gokeyring.ErrNotFound) {
		return "", secret.ErrNotFound
	}
	if err != nil {
		return "", errz.Err(err)
	}
	return v, nil
}

// Set writes value to the keyring at path, overwriting any existing entry.
func (s *Store) Set(_ context.Context, path, value string) error {
	return errz.Err(gokeyring.Set(Service, path, value))
}

// Delete removes the keyring entry at path. Deleting a non-existent
// entry is not an error.
//
// The order here matters subtly: errz.Err wraps before the errors.Is
// check, but errz wrappers expose Unwrap so the gokeyring.ErrNotFound
// sentinel is still matched through the chain. Don't invert this
// without confirming the wrapper preserves the comparison.
func (s *Store) Delete(_ context.Context, path string) error {
	err := errz.Err(gokeyring.Delete(Service, path))
	if errors.Is(err, gokeyring.ErrNotFound) {
		return nil
	}
	return err
}
