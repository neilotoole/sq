// Package op is the 1Password CLI backend for libsq/core/secret. A
// ${op://<vault>/<item>/[<section>/]<field>} placeholder resolves to the
// value 1Password's "op" CLI returns for that secret reference. The path
// is passed through to "op read" verbatim; sq does not parse it.
//
// Auth is "op"'s problem: the user must already be signed in (biometric,
// op signin, or a service-account token in OP_SERVICE_ACCOUNT_TOKEN).
// sq surfaces "op"'s stderr in the wrapped error when authentication or
// connectivity fails. Read-only: sq never writes to 1Password.
//
// Minimum supported "op" version: v2 (the version where "op read" landed).
package op

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// Resolver implements secret.Resolver by shelling out to the 1Password
// "op" CLI. A single Resolver caches successful resolutions for its
// lifetime so repeated placeholders within one sq invocation only call
// "op" once per path. Concurrent Resolve calls for the same path are
// coalesced via singleflight so the user never sees duplicate biometric
// prompts.
type Resolver struct {
	flight singleflight.Group
	cache  sync.Map // path -> string
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
//
// Concurrent callers for the same path share one in-flight "op read"
// invocation; a transient failure (network blip, op timeout) is replayed
// to every waiter in that flight, but is not cached, so a sequential
// retry reaches op again.
func (r *Resolver) Resolve(ctx context.Context, path string) (string, error) {
	if v, ok := r.cache.Load(path); ok {
		return v.(string), nil
	}
	// DoChan (not Do) so each caller can honor its own ctx while waiting:
	// the shared "op read" runs with the first caller's context, but later
	// callers with a tighter deadline can abort independently without
	// affecting the in-flight invocation.
	ch := r.flight.DoChan(path, func() (any, error) {
		// Re-check the cache: a concurrent flight may have populated it
		// while this caller was waiting on the singleflight lock.
		if v, ok := r.cache.Load(path); ok {
			return v.(string), nil
		}
		return r.runOpRead(ctx, path)
	})
	select {
	case res := <-ch:
		if res.Err != nil {
			return "", res.Err
		}
		return res.Val.(string), nil
	case <-ctx.Done():
		return "", errz.Wrapf(ctx.Err(), "op read %s", path)
	}
}

// runOpRead is the cache-miss path: locate "op", run
// "op read op://<rest>" (where path already begins with "//"),
// classify the result, and on success populate the cache and return
// the trimmed value.
func (r *Resolver) runOpRead(ctx context.Context, path string) (string, error) {
	opPath, err := exec.LookPath("op")
	if err != nil {
		return "", errz.Wrap(err,
			"1Password 'op' CLI not found on PATH; install it from "+
				"https://developer.1password.com/docs/cli/get-started/")
	}

	// opPath is the absolute path resolved by LookPath for the literal
	// "op", and the read URI is passed as a separate argv entry, not
	// through a shell, so a hostile placeholder body cannot break out
	// into command injection.
	cmd := exec.CommandContext(ctx, opPath, "read", "op:"+path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Honor context cancellation before any stderr-based classification:
		// when the child is killed by ctx, stderr is typically empty.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", errz.Wrapf(ctxErr, "op read %s", path)
		}
		// Trim only trailing newlines so op's stderr is preserved
		// otherwise verbatim in the wrapped error message.
		stderrStr := strings.TrimRight(stderr.String(), "\r\n")
		if isNotFoundStderr(stderrStr) {
			// Bare sentinel, matching env/file/keyring; expand.go adds
			// "resolve ${op:<path>}" context at the outer layer.
			return "", secret.ErrNotFound
		}
		if stderrStr == "" {
			return "", errz.Wrapf(err, "op read %s", path)
		}
		return "", errz.Wrapf(err, "op read %s: %s", path, stderrStr)
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
// Matching is a small, conservative set kept item-scoped on purpose:
// generic markers like "couldn't find" would also match account / vault
// access failures and silently misclassify them as not-found. Unknown
// error text is surfaced verbatim rather than misclassified.
var notFoundMarkers = []string{
	"isn't an item",
	"item not found",
	"no item found",
	"couldn't find the item",
	"couldn't find an item",
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
