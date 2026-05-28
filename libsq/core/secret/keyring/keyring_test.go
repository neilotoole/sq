package keyring_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/keyring"
)

func TestResolver_SetGetDelete(t *testing.T) {
	gokeyring.MockInit() // in-memory backend for tests

	r := keyring.New()
	ctx := context.Background()

	const path = "@sakila/password"
	const value = "hunter2"

	require.NoError(t, r.Set(ctx, path, value))

	got, err := r.Resolve(ctx, path)
	require.NoError(t, err)
	require.Equal(t, value, got)

	require.NoError(t, r.Delete(ctx, path))

	_, err = r.Resolve(ctx, path)
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestResolver_OverwriteOnSet(t *testing.T) {
	gokeyring.MockInit()
	r := keyring.New()
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "k", "first"))
	require.NoError(t, r.Set(ctx, "k", "second"))

	got, err := r.Resolve(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "second", got)
}

func TestResolver_DeleteMissingIsNotError(t *testing.T) {
	gokeyring.MockInit()
	r := keyring.New()
	// Deleting a non-existent entry should not error (idempotent).
	require.NoError(t, r.Delete(context.Background(), "no-such-entry"))
}
