package testrun_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/testrun"
)

// TestNew_ConfigDirRemoved verifies that the config dir created by
// testrun.New is removed when the test passes (gh #1162).
func TestNew_ConfigDirRemoved(t *testing.T) {
	var cfgDir string
	// If the inner subtest fails, stop here instead of falling through to the
	// NoDirExists assertion below, which would add a second, misleading
	// failure: the inner subtest's dir is kept on failure, by design.
	if !t.Run("run", func(t *testing.T) {
		tr := testrun.New(context.Background(), t, nil)
		store, ok := tr.Run.ConfigStore.(*yamlstore.Store)
		require.True(t, ok, "config store is %T", tr.Run.ConfigStore)
		cfgDir = filepath.Dir(store.Path)
		require.DirExists(t, cfgDir)
	}) {
		t.FailNow()
	}

	require.NotEmpty(t, cfgDir)
	require.NoDirExists(t, cfgDir)
}
