// The tests in this file install a POSIX shell stub on PATH to drive the
// op resolver without a live 1Password binary. The stub is a "#!/bin/sh"
// script and is therefore not executable on Windows (PATHEXT does not
// match "op", and Windows cannot exec a shebang). The op resolver itself
// is platform-independent; only this test harness is Unix-only.
//go:build !windows

package op_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/op"
)

// installStubOp writes a POSIX-shell stub binary named "op" into a fresh
// t.TempDir and prepends that dir to PATH for the duration of t. The stub's
// behavior is selected by the SQ_TEST_OP_MODE env var (set by each test via
// t.Setenv):
//
//	value     -> prints $SQ_TEST_OP_VALUE exactly (no added newline), exit 0
//	echo      -> prints argv[2] then "\n", exit 0 (used to assert pass-through)
//	notfound  -> writes "isn't an item" to stderr, exit 1
//	stderr    -> writes $SQ_TEST_OP_STDERR verbatim to stderr, exit 1
//	unauthed  -> writes a 1Password sign-in message to stderr, exit 1
//	hang      -> touches $SQ_TEST_OP_STARTED_FILE (if set) then sleeps 60s
//	count     -> appends "x" to $SQ_TEST_OP_COUNT_FILE then prints $SQ_TEST_OP_VALUE with "\n", exit 0
func installStubOp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$SQ_TEST_OP_MODE" in
  value)    printf '%s' "$SQ_TEST_OP_VALUE" ;;
  echo)     printf '%s\n' "$2" ;;
  notfound) echo "[ERROR] \"$2\" isn't an item. Specify the item by its name or ID." >&2; exit 1 ;;
  stderr)   printf '%s\n' "$SQ_TEST_OP_STDERR" >&2; exit 1 ;;
  unauthed) echo "[ERROR] you aren't signed in. Run 'op signin' to sign in." >&2; exit 1 ;;
  hang)     if [ -n "$SQ_TEST_OP_STARTED_FILE" ]; then : >"$SQ_TEST_OP_STARTED_FILE"; fi; sleep 60 ;;
  count)    printf 'x' >>"$SQ_TEST_OP_COUNT_FILE"; printf '%s\n' "$SQ_TEST_OP_VALUE" ;;
  *)        echo "stub op: unknown mode '$SQ_TEST_OP_MODE'" >&2; exit 2 ;;
esac
`
	path := filepath.Join(dir, "op")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestResolver_HappyPath(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "value")
	t.Setenv("SQ_TEST_OP_VALUE", "hunter2")

	got, err := op.NewResolver().Resolve(context.Background(), "//Private/sakila/password")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_PassesThroughURI(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "echo")

	got, err := op.NewResolver().Resolve(context.Background(), "//Private/sakila/dsn")
	require.NoError(t, err)
	// The stub echoes shell $2, which is the single argument to the
	// "read" subcommand. The resolver must reconstitute the full op://
	// URI from the path (which already begins with "//"). A naive
	// "op://"+path would yield op:////Private/...; this asserts the
	// bug is absent.
	require.Equal(t, "op://Private/sakila/dsn", got)
}

func TestResolver_NotFound(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "notfound")

	_, err := op.NewResolver().Resolve(context.Background(), "//Private/nope/field")
	require.ErrorIs(t, err, secret.ErrNotFound)
}

// TestResolver_NotFoundMarkers drives every entry of the resolver's
// notFoundMarkers list against the classifier, plus a confounding
// auth-style message that must NOT classify as not-found.
func TestResolver_NotFoundMarkers(t *testing.T) {
	cases := []struct {
		name, stderr string
		wantNotFound bool
	}{
		{"isn't_an_item", "[ERROR] \"op://v/i/f\" isn't an item.", true},
		{"item_not_found", "ERROR: item not found", true},
		{"no_item_found", "ERROR: no item found with that name", true},
		{"couldnt_find_the_item", "[ERROR] couldn't find the item", true},
		{"couldnt_find_an_item", "[ERROR] couldn't find an item matching that query", true},
		{"mixed_case", "ERROR: ISN'T AN ITEM matches the lowercase marker", true},

		// Negative: an auth-style "Couldn't find your account" must not
		// be misclassified as ErrNotFound. This guards against the
		// previous over-broad "couldn't find" marker regressing.
		{"couldnt_find_account", "[ERROR] Couldn't find your account. Run 'op signin'.", false},
		{"network", "[ERROR] connection refused dialing https://my.1password.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installStubOp(t)
			t.Setenv("SQ_TEST_OP_MODE", "stderr")
			t.Setenv("SQ_TEST_OP_STDERR", tc.stderr)

			_, err := op.NewResolver().Resolve(context.Background(), "//v/i/f")
			require.Error(t, err)
			if tc.wantNotFound {
				require.ErrorIs(t, err, secret.ErrNotFound,
					"stderr %q must classify as ErrNotFound", tc.stderr)
			} else {
				require.NotErrorIs(t, err, secret.ErrNotFound,
					"stderr %q must NOT classify as ErrNotFound", tc.stderr)
				require.Contains(t, err.Error(), strings.TrimSpace(tc.stderr),
					"stderr must be surfaced verbatim in the error message")
			}
		})
	}
}

func TestResolver_TrimsTrailingNewline(t *testing.T) {
	cases := []struct {
		name, value, want string
	}{
		{"lf", "hunter2\n", "hunter2"},
		{"crlf", "hunter2\r\n", "hunter2"},
		{"no_trailing", "hunter2", "hunter2"},
		{"double_lf", "hunter2\n\n", "hunter2\n"}, // only one trim
		{"embedded_lf", "line1\nline2\n", "line1\nline2"},
		{"bare_cr", "hunter2\r", "hunter2\r"}, // bare CR is preserved
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installStubOp(t)
			t.Setenv("SQ_TEST_OP_MODE", "value")
			t.Setenv("SQ_TEST_OP_VALUE", tc.value)

			got, err := op.NewResolver().Resolve(context.Background(), "//v/i/f")
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolver_CachesAcrossCalls(t *testing.T) {
	installStubOp(t)
	countFile := filepath.Join(t.TempDir(), "calls")
	t.Setenv("SQ_TEST_OP_MODE", "count")
	t.Setenv("SQ_TEST_OP_COUNT_FILE", countFile)
	t.Setenv("SQ_TEST_OP_VALUE", "hunter2")

	r := op.NewResolver()
	for range 3 {
		got, err := r.Resolve(context.Background(), "//v/i/f")
		require.NoError(t, err)
		require.Equal(t, "hunter2", got)
	}

	data, err := os.ReadFile(countFile)
	require.NoError(t, err)
	require.Equal(t, "x", string(data),
		"three Resolve calls for the same path must invoke op exactly once")
}

func TestResolver_CacheIsKeyedByPath(t *testing.T) {
	installStubOp(t)
	countFile := filepath.Join(t.TempDir(), "calls")
	t.Setenv("SQ_TEST_OP_MODE", "count")
	t.Setenv("SQ_TEST_OP_COUNT_FILE", countFile)
	t.Setenv("SQ_TEST_OP_VALUE", "hunter2")

	r := op.NewResolver()
	_, err := r.Resolve(context.Background(), "//v/i1/f")
	require.NoError(t, err)
	_, err = r.Resolve(context.Background(), "//v/i2/f")
	require.NoError(t, err)

	data, err := os.ReadFile(countFile)
	require.NoError(t, err)
	require.Equal(t, "xx", string(data),
		"distinct paths must each invoke op")
}

// TestResolver_ErrorsAreNotCached pins the contract that failed
// resolutions do not poison the cache: a subsequent retry must reach op
// again rather than replaying the prior error.
func TestResolver_ErrorsAreNotCached(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "notfound")

	r := op.NewResolver()
	_, err := r.Resolve(context.Background(), "//v/i/f")
	require.ErrorIs(t, err, secret.ErrNotFound)

	// Switch the stub to success. If errors were cached, this call
	// would return the prior ErrNotFound without re-invoking op.
	t.Setenv("SQ_TEST_OP_MODE", "value")
	t.Setenv("SQ_TEST_OP_VALUE", "hunter2")

	got, err := r.Resolve(context.Background(), "//v/i/f")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_ContextCancellation(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "hang")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := op.NewResolver().Resolve(ctx, "//v/i/f")
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, 2*time.Second,
		"cancellation should kill the op process promptly")
	require.NotErrorIs(t, err, secret.ErrNotFound,
		"cancellation must not be misclassified as not-found")
	require.True(t,
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"error should chain to a context sentinel; got %v", err)
}

// TestResolver_PerCallerCtxCancel verifies that a caller waiting on a
// coalesced singleflight can bail when its own context expires, even
// when the shared "op read" is still hanging.
func TestResolver_PerCallerCtxCancel(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "hang")

	startedFile := filepath.Join(t.TempDir(), "started")
	t.Setenv("SQ_TEST_OP_STARTED_FILE", startedFile)

	r := op.NewResolver()
	// First call uses the test-scoped context and is intentionally not
	// awaited; it pins the in-flight "op read" inside the singleflight
	// closure so the second caller is forced to wait. t.Context() is
	// cancelled at test cleanup, which kills the hanging op process.
	leaderCtx := t.Context()
	go func() {
		_, _ = r.Resolve(leaderCtx, "//v/i/f")
	}()

	// Wait deterministically for the leader to enter singleflight: the
	// stub touches startedFile right before sleeping.
	require.Eventually(t, func() bool {
		_, err := os.Stat(startedFile)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "leader op stub never started")

	// Second caller has a tight deadline; it must abort on its own ctx
	// without waiting for the leader's hang to complete.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := r.Resolve(ctx, "//v/i/f")
	elapsed := time.Since(start)
	require.Error(t, err)
	require.Less(t, elapsed, time.Second,
		"follower must bail on its own ctx, not wait for leader")
	require.True(t,
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"follower error should chain to a context sentinel; got %v", err)
}

func TestResolver_OpBinaryMissing(t *testing.T) {
	// Point PATH at an empty directory so "op" is not found anywhere.
	t.Setenv("PATH", t.TempDir())

	_, err := op.NewResolver().Resolve(context.Background(), "//v/i/f")
	require.Error(t, err)
	require.NotErrorIs(t, err, secret.ErrNotFound)
	msg := err.Error()
	require.Contains(t, msg, "op",
		"error must mention the missing binary by name")
	require.Contains(t, strings.ToLower(msg), "1password",
		"error hint should point the user at 1Password")
}

func TestResolver_UnauthedSurfacesStderr(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "unauthed")

	_, err := op.NewResolver().Resolve(context.Background(), "//Private/sakila/dsn")
	require.Error(t, err)
	require.NotErrorIs(t, err, secret.ErrNotFound,
		"sign-in failures must not be misclassified as not-found")
	require.Contains(t, err.Error(), "you aren't signed in",
		"the op CLI stderr must be surfaced so the user knows what to do")
}
