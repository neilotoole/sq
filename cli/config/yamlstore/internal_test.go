package yamlstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh/tu"
)

// setBuildVersion sets the build version for the lifecycle of test t.
// This is a local copy to avoid import cycle with testh package.
func setBuildVersion(tb testing.TB, vers string) {
	tb.Helper()
	prevVers := buildinfo.Version
	tb.Setenv(buildinfo.EnvOverrideVersion, vers)
	buildinfo.Version = vers
	tb.Cleanup(func() {
		buildinfo.Version = prevVers
	})
}

func Test_getConfigDirFromFlag(t *testing.T) {
	testCases := []struct {
		in      []string
		want    string
		wantOK  bool
		wantErr bool
	}{
		{in: nil, wantOK: false},
		{in: []string{}, wantOK: false},
		{in: []string{"sq", "--" + flag.Config + "=/home"}, wantOK: true, want: "/home"},
		{in: []string{"sq", "--" + flag.Config, "/home"}, wantOK: true, want: "/home"},
		{in: []string{"sq", "some", "args"}, wantOK: false, want: ""},
		{in: []string{"sq", "--some=value", "args"}, wantOK: false, want: ""},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.in, " ")), func(t *testing.T) {
			got, gotOK, gotErr := getConfigDirFromFlag(tc.in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			if !tc.wantOK {
				require.False(t, gotOK)
				require.Empty(t, got)
				return
			}

			require.True(t, tc.wantOK)
			require.Equal(t, tc.want, got)
		})
	}
}

// Test_checkNeedsUpgrade_newerConfigVersion verifies that checkNeedsUpgrade
// returns errConfigVersionNewerThanBuild when the config version is newer
// than the (non-prerelease) build version.
func Test_checkNeedsUpgrade_newerConfigVersion(t *testing.T) {
	// Set a non-prerelease build version.
	setBuildVersion(t, "v0.48.0")

	// Create temp config with newer version.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "sq.yml")
	err := os.WriteFile(cfgPath, []byte("config.version: v99.0.0\n"), 0o644)
	require.NoError(t, err)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	// Should return errConfigVersionNewerThanBuild.
	needsUpgrade, foundVers, err := checkNeedsUpgrade(ctx, cfgPath)
	require.ErrorIs(t, err, errConfigVersionNewerThanBuild)
	require.False(t, needsUpgrade)
	require.Equal(t, "v99.0.0", foundVers)
}

// Test_checkNeedsUpgrade_newerConfigVersion_prerelease verifies that
// checkNeedsUpgrade does NOT return an error when the build version is a
// prerelease, even if the config version is newer. Prerelease builds are
// exempt from the version check.
func Test_checkNeedsUpgrade_newerConfigVersion_prerelease(t *testing.T) {
	// Prerelease builds should NOT error.
	setBuildVersion(t, "v0.48.0-dev")

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "sq.yml")
	err := os.WriteFile(cfgPath, []byte("config.version: v99.0.0\n"), 0o644)
	require.NoError(t, err)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	// Should NOT error for prerelease.
	needsUpgrade, foundVers, err := checkNeedsUpgrade(ctx, cfgPath)
	require.NoError(t, err)
	require.False(t, needsUpgrade)
	require.Equal(t, "v99.0.0", foundVers)
}

// Test_Store_Load_NoBackupWhenNoUpgradeFuncs verifies that a routine
// version bump with no registered upgrade funcs in range does not
// write a pre-upgrade backup file. Due to version inflation (see the
// IMPLEMENTATION NOTE on errConfigVersionNewerThanBuild), doUpgrade
// runs on every sq release; the backup must be written only when an
// upgrade func actually transforms the config, else one
// credential-bearing backup accumulates per release.
func Test_Store_Load_NoBackupWhenNoUpgradeFuncs(t *testing.T) {
	setBuildVersion(t, "v0.48.0")

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "sq.yml")
	err := os.WriteFile(cfgPath, []byte("config.version: v0.47.0\n"), 0o644)
	require.NoError(t, err)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		// Non-nil but empty: the upgrade path runs, with zero funcs in
		// range. This mirrors production for any release that doesn't
		// change the config schema.
		UpgradeRegistry: UpgradeRegistry{},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.48.0", cfg.Version, "config.version is still stamped")

	entries, err := os.ReadDir(cfgDir)
	require.NoError(t, err)
	for _, e := range entries {
		require.NotContains(t, e.Name(), ".bak.",
			"no backup file expected when no upgrade funcs ran")
	}
}

// Test_Store_Load_newerConfigVersion verifies that Store.Load succeeds
// (returns no error) when the config version is newer than the build version.
// The sentinel error errConfigVersionNewerThanBuild should be handled
// internally, logging a warning but allowing the config to load.
func Test_Store_Load_newerConfigVersion(t *testing.T) {
	// Set a non-prerelease build version.
	setBuildVersion(t, "v0.48.0")

	// Create a minimal valid config with a newer version.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "sq.yml")
	cfgContent := `config.version: v99.0.0
`
	err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644)
	require.NoError(t, err)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{},
	}

	// Load should succeed despite the config version being newer.
	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "v99.0.0", cfg.Version)
}
