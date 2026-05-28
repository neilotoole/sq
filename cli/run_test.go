package cli_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// TestRun_SecretRegistry_HasKeyring verifies that preRun initializes
// SecretRegistry on the Run and registers the "keyring" scheme.
func TestRun_SecretRegistry_HasKeyring(t *testing.T) {
	// Use the in-memory mock backend so tests never touch the real OS keyring.
	gokeyring.MockInit()

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil).Hush()

	// Execute a lightweight built-in command to trigger preRun.
	require.NoError(t, tr.Exec("version", "--json"))

	require.NotNil(t, tr.Run.SecretRegistry)

	// "keyring" scheme must be registered. Resolving an unknown path returns
	// a non-ErrUnknownScheme error, proving the scheme dispatcher fired.
	_, err := tr.Run.SecretRegistry.ResolveScheme(ctx, "keyring", "no-such-entry")
	require.Error(t, err)
	require.NotErrorIs(t, err, secret.ErrUnknownScheme)
}
