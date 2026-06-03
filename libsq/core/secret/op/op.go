// Package op is the 1Password CLI backend for libsq/core/secret. A
// ${op://<vault>/<item>/[<section>/]<field>} placeholder resolves to the
// value 1Password's "op" CLI returns for that secret reference. The path
// is passed through to "op read" verbatim; sq does not parse it.
//
// Auth is "op"'s problem: the user must already be signed in (op signin,
// biometric prompt, or a service-account token in OP_SERVICE_ACCOUNT_TOKEN).
// sq surfaces "op"'s stderr verbatim when authentication or connectivity
// fails. Read-only: sq never writes to 1Password.
//
// Minimum supported "op" version: v2 (the version where "op read" landed).
package op

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// Resolver implements secret.Resolver by shelling out to the 1Password
// "op" CLI. A single Resolver caches successful resolutions for its
// lifetime so repeated placeholders within one sq invocation only call
// "op" once per path.
type Resolver struct {
	cache sync.Map // path -> string
}

// NewResolver returns a Resolver. Callers register the result with a
// secret.Registry under the "op" scheme.
func NewResolver() *Resolver {
	return &Resolver{}
}

// Resolve invokes "op read" for the placeholder body path (which includes
// the leading "//", e.g. "//Private/sakila/dsn"). The "op:" scheme is
// reattached, so "op" receives the full URI "op://Private/sakila/dsn".
//
// Successful values are cached per-instance. A single trailing LF or CRLF
// is trimmed from the output, matching 1Password CLI behavior and the
// secret/file resolver convention.
func (r *Resolver) Resolve(ctx context.Context, path string) (string, error) {
	if v, ok := r.cache.Load(path); ok {
		return v.(string), nil
	}

	// G204: binary name "op" is a literal; path comes from sq's own YAML
	// config (a placeholder body), so this is not an untrusted external input.
	cmd := exec.CommandContext(ctx, "op", "read", "op:"+path) //nolint:gosec // G204: see comment above.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errz.Wrap(err, "op read")
	}

	out := strings.TrimRight(stdout.String(), "\n")
	out = strings.TrimRight(out, "\r")
	r.cache.Store(path, out)
	// T4 maps op's "not an item" stderr to secret.ErrNotFound; keep the import live.
	_ = secret.ErrNotFound
	return out, nil
}
