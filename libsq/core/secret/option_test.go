package secret_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
)

func TestOptSecretsStore_DefaultIsInline(t *testing.T) {
	require.Equal(t, "inline", secret.OptSecretsStore.Default())
}

func TestOptSecretsStore_KeyringIsValid(t *testing.T) {
	v, err := secret.OptSecretsStore.Process(options.Options{
		"secrets.store": "keyring",
	})
	require.NoError(t, err)
	require.Equal(t, "keyring", v["secrets.store"])
}

func TestOptSecretsStore_InlineIsValid(t *testing.T) {
	v, err := secret.OptSecretsStore.Process(options.Options{
		"secrets.store": "inline",
	})
	require.NoError(t, err)
	require.Equal(t, "inline", v["secrets.store"])
}

func TestOptSecretsStore_RejectsUnknown(t *testing.T) {
	_, err := secret.OptSecretsStore.Process(options.Options{
		"secrets.store": "bogus",
	})
	require.Error(t, err)
}
