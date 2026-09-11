package secret_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/core/secret/env"
)

func TestRegistrySchemes(t *testing.T) {
	reg := secret.NewRegistry()
	require.Empty(t, reg.Schemes(), "empty registry has no schemes")

	// Register out of alphabetical order; Schemes must return them sorted.
	reg.Register("keyring", env.NewResolver())
	reg.Register("env", env.NewResolver())
	reg.Register("file", env.NewResolver())

	require.Equal(t, []string{"env", "file", "keyring"}, reg.Schemes())
}
