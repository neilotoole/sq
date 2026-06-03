package op_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret/op"
)

// installStubOp writes a POSIX-shell stub binary named "op" into a fresh
// t.TempDir and prepends that dir to PATH for the duration of t. The stub's
// behavior is selected by the SQ_TEST_OP_MODE env var (set by each test via
// t.Setenv):
//
//	value    -> prints $SQ_TEST_OP_VALUE then "\n", exit 0
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
  value)    printf '%s\n' "$SQ_TEST_OP_VALUE" ;;
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
