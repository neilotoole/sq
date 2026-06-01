// Package keyring is the OS-keyring backend for libsq/core/secret.
// It wraps github.com/zalando/go-keyring, which supports macOS Keychain,
// Windows Credential Manager, and freedesktop Secret Service on Linux.
package keyring

import (
	"context"
	"errors"

	"github.com/neilotoole/sq/libsq/core/errz"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// Service is the fixed keyring service name used for all sq secrets.
const Service = "sq"

// Resolver implements secret.Resolver against the OS keyring.
type Resolver struct{}

// New returns a Resolver. Callers register the result with a
// secret.Registry under the "keyring" scheme.
func New() *Resolver {
	return &Resolver{}
}

// Resolve returns the secret stored at path. Returns secret.ErrNotFound
// when no entry exists.
func (r *Resolver) Resolve(_ context.Context, path string) (string, error) {
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
func (r *Resolver) Set(_ context.Context, path, value string) error {
	return errz.Err(gokeyring.Set(Service, path, value))
}

// Delete removes the keyring entry at path. Deleting a non-existent
// entry is not an error.
func (r *Resolver) Delete(_ context.Context, path string) error {
	err := errz.Err(gokeyring.Delete(Service, path))
	if errors.Is(err, gokeyring.ErrNotFound) {
		return nil
	}
	return err
}
