package env_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/env"
)

func TestResolver_Set(t *testing.T) {
	t.Setenv("SQ_TEST_RESOLVE_ME", "hunter2")

	got, err := env.New().Resolve(context.Background(), "SQ_TEST_RESOLVE_ME")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestResolver_Unset(t *testing.T) {
	// Explicitly ensure the var is not set.
	_, err := env.New().Resolve(context.Background(), "SQ_TEST_DEFINITELY_NOT_SET_X9F2K")
	require.ErrorIs(t, err, secret.ErrNotFound)
}

func TestResolver_EmptyStringIsSet(t *testing.T) {
	t.Setenv("SQ_TEST_EMPTY_VAR", "")

	got, err := env.New().Resolve(context.Background(), "SQ_TEST_EMPTY_VAR")
	require.NoError(t, err)
	require.Equal(t, "", got)
}
