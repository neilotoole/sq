package secret_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
)

func TestOptSecretsDefault_DefaultIsInline(t *testing.T) {
	require.Equal(t, "inline", secret.OptSecretsDefault.Default())
}

func TestOptSecretsDefault_KeyringIsValid(t *testing.T) {
	v, err := secret.OptSecretsDefault.Process(options.Options{
		"secrets.default": "keyring",
	})
	require.NoError(t, err)
	require.Equal(t, "keyring", v["secrets.default"])
}

func TestOptSecretsDefault_InlineIsValid(t *testing.T) {
	v, err := secret.OptSecretsDefault.Process(options.Options{
		"secrets.default": "inline",
	})
	require.NoError(t, err)
	require.Equal(t, "inline", v["secrets.default"])
}

func TestOptSecretsDefault_RejectsUnknown(t *testing.T) {
	_, err := secret.OptSecretsDefault.Process(options.Options{
		"secrets.default": "bogus",
	})
	require.Error(t, err)
}
