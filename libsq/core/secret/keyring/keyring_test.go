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
