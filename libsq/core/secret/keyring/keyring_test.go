package keyring_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func TestStore_SetGetDelete(t *testing.T) {
	gokeyring.MockInit() // in-memory backend for tests

	r := keyring.NewStore()
	ctx := context.Background()

	const path = "my_db_pw"
	const value = "hunter2"

	require.NoError(t, r.Set(ctx, path, value))

	got, err := r.Resolve(ctx, path)
	require.NoError(t, err)
	require.Equal(t, value, got)

	require.NoError(t, r.Delete(ctx, path))

	_, err = r.Resolve(ctx, path)
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestStore_OverwriteOnSet(t *testing.T) {
	gokeyring.MockInit()
	r := keyring.NewStore()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "k", "first"))
	require.NoError(t, r.Set(ctx, "k", "second"))

	got, err := r.Resolve(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "second", got)
}

func TestStore_DeleteMissingIsNotError(t *testing.T) {
	gokeyring.MockInit()
	r := keyring.NewStore()
	// Deleting a non-existent entry should not error (idempotent).
	require.NoError(t, r.Delete(context.Background(), "no-such-entry"))
}

// countingStore wraps a secret.Resolver, counting Resolve invocations.
type countingStore struct {
	inner secret.Resolver
	count int
}

func (c *countingStore) Resolve(ctx context.Context, path string) (string, error) {
	c.count++
	return c.inner.Resolve(ctx, path)
}

// TestStore_RegistryMemoizesKeyringResolution verifies that a
// keyring-backed Registry hits the OS keyring once per path per run
// (gh #779). Each Store.Resolve is an OS-keychain IPC roundtrip, so
// repeated resolution of the same placeholder (e.g. one Grips.Open per
// table during inspect) must be served from the Registry memo. The
// memoization deliberately lives in secret.Registry rather than in
// Store itself: the keyring write commands (create, update, rm,
// migrate) use Store directly and must always read through to the
// backend.
func TestStore_RegistryMemoizesKeyringResolution(t *testing.T) {
	gokeyring.MockInit()
	ctx := context.Background()

	store := keyring.NewStore()
	require.NoError(t, store.Set(ctx, "my_db_pw", "hunter2"))

	counter := &countingStore{inner: store}
	reg := secret.NewRegistry()
	reg.Register("keyring", counter)

	for range 3 {
		got, err := reg.Expand(ctx, "postgres://alice:${keyring:my_db_pw}@db/sakila")
		require.NoError(t, err)
		require.Equal(t, "postgres://alice:hunter2@db/sakila", got)
	}
	require.Equal(t, 1, counter.count,
		"keyring backend must be hit once per path per run")
}
