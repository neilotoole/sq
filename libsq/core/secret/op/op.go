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

	if _, err := exec.LookPath("op"); err != nil {
		return "", errz.Wrap(err,
			"1Password 'op' CLI not found on PATH; install it from "+
				"https://developer.1password.com/docs/cli/get-started/")
	}

	// G204: binary name "op" is a literal; path comes from sq's own YAML
	// config (a placeholder body), so this is not an untrusted external input.
	cmd := exec.CommandContext(ctx, "op", "read", "op:"+path) //nolint:gosec // G204: see comment above.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if isNotFoundStderr(stderr.String()) {
			return "", secret.ErrNotFound
		}
		return "", errz.Wrapf(err, "op read: %s", strings.TrimSpace(stderr.String()))
	}

	out := stdout.String()
	switch {
	case strings.HasSuffix(out, "\r\n"):
		out = out[:len(out)-2]
	case strings.HasSuffix(out, "\n"):
		out = out[:len(out)-1]
	}
	r.cache.Store(path, out)
	return out, nil
}

// notFoundMarkers are the stderr substrings 1Password's "op" CLI uses to
// indicate the referenced item, vault, section, or field does not exist.
// Matching is a small, conservative set; unknown error text is surfaced
// verbatim rather than misclassified.
var notFoundMarkers = []string{
	"isn't an item",
	"item not found",
	"no item found",
	"couldn't find",
}

func isNotFoundStderr(s string) bool {
	s = strings.ToLower(s)
	for _, m := range notFoundMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
