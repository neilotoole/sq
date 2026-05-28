// Package env is the environment-variable backend for libsq/core/secret.
// A ${env:VAR_NAME} placeholder resolves to os.Getenv("VAR_NAME").
package env

import (
	"context"
	"os"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// Resolver implements secret.Resolver against the process environment.
type Resolver struct{}

// New returns a Resolver. Callers register the result with a
// secret.Registry under the "env" scheme.
func New() *Resolver {
	return &Resolver{}
}

// Resolve returns the value of environment variable name. Returns
// secret.ErrNotFound when the variable is not set. A variable set to
// the empty string is returned as "" with a nil error (the caller may
// distinguish empty from missing via the error).
func (r *Resolver) Resolve(_ context.Context, name string) (string, error) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return "", secret.ErrNotFound
	}
	return v, nil
}
