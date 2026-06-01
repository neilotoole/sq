package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret"
)

// TestExportExpandConfig_NilCollection guards against a panic when
// Config.Collection is nil. The runtime path through ExecuteWith
// rejects nil Collection earlier in FinishRunInit, but the function-
// level guard keeps exportExpandConfig safe if called from any future
// caller that doesn't go through the full run init.
func TestExportExpandConfig_NilCollection(t *testing.T) {
	cfg := &config.Config{Version: "v0.0.0-dev"}
	ru := &run.Run{SecretRegistry: secret.NewRegistry()}

	got, err := exportExpandConfig(context.Background(), ru, cfg)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Nil(t, got.Collection)
}
