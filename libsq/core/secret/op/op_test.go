package op_test

import (
	"context"
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
//	value    -> prints $SQ_TEST_OP_VALUE exactly (no added newline), exit 0
//	echo     -> prints argv[2] then "\n", exit 0 (used to assert pass-through)
//	notfound -> writes a 1Password "not an item" message to stderr, exit 1
//	unauthed -> writes a 1Password sign-in message to stderr, exit 1
//	hang     -> sleeps 60s (used for cancellation tests)
//	count    -> appends "x" to $SQ_TEST_OP_COUNT_FILE then prints value
func installStubOp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
case "$SQ_TEST_OP_MODE" in
  value)    printf '%s' "$SQ_TEST_OP_VALUE" ;;
  echo)     printf '%s\n' "$2" ;;
  notfound) echo "[ERROR] \"$2\" isn't an item. Specify the item by its name or ID." >&2; exit 1 ;;
  unauthed) echo "[ERROR] you aren't signed in. Run 'op signin' to sign in." >&2; exit 1 ;;
  hang)     sleep 60 ;;
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
	// The stub echoes argv[2], which is the second positional arg to
	// "op read". The resolver must reconstitute the full op:// URI from
	// the path (which already begins with "//"). A naive "op://"+path
	// would yield op:////Private/...; this asserts the bug is absent.
	require.Equal(t, "op://Private/sakila/dsn", got)
}

func TestResolver_NotFound(t *testing.T) {
	installStubOp(t)
	t.Setenv("SQ_TEST_OP_MODE", "notfound")

	_, err := op.NewResolver().Resolve(context.Background(), "//Private/nope/field")
	require.ErrorIs(t, err, secret.ErrNotFound)
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
