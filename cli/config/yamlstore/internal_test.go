package yamlstore

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/config"
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

// writeTestConfig writes content to sq.yml in a temp dir, returning
// the config path.
func writeTestConfig(t *testing.T, content string) (cfgDir, cfgPath string) {
	t.Helper()
	cfgDir = t.TempDir()
	cfgPath = filepath.Join(cfgDir, "sq.yml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
	return cfgDir, cfgPath
}

// requireNoBackupFiles asserts that dir contains no ".bak." files.
func requireNoBackupFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		require.NotContains(t, e.Name(), ".bak.",
			"no backup file expected")
	}
}

// Test_checkNeedsUpgrade_newerConfigVersion verifies that checkNeedsUpgrade
// returns errConfigVersionNewerThanBuild when the config version is newer
// than the (non-prerelease) build version.
func Test_checkNeedsUpgrade_newerConfigVersion(t *testing.T) {
	// Set a non-prerelease build version.
	setBuildVersion(t, "v0.48.0")

	// Create temp config with newer version.
	_, cfgPath := writeTestConfig(t, "config.version: v99.0.0\n")

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, UpgradeRegistry: UpgradeRegistry{}}

	// Should return errConfigVersionNewerThanBuild.
	needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx)
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

	_, cfgPath := writeTestConfig(t, "config.version: v99.0.0\n")

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, UpgradeRegistry: UpgradeRegistry{}}

	// Should NOT error for prerelease.
	needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx)
	require.NoError(t, err)
	require.False(t, needsUpgrade)
	require.Equal(t, "v99.0.0", foundVers)
}

// Test_checkNeedsUpgrade_schemaVersion verifies that needsUpgrade is
// determined by comparing the config version against the highest
// version in the UpgradeRegistry (the schema version), not against
// the build version.
func Test_checkNeedsUpgrade_schemaVersion(t *testing.T) {
	noop := func(_ context.Context, before []byte) ([]byte, error) {
		return before, nil
	}

	testCases := []struct {
		name            string
		buildVers       string
		cfgVers         string
		registry        UpgradeRegistry
		wantNeeds       bool
		wantNewerThanBd bool
	}{
		{
			// A release without schema changes: config carries an
			// inflated (binary-stamped) version above the highest
			// registered upgrade. No upgrade, no error.
			name:      "inflated version tolerated",
			buildVers: "v0.55.0",
			cfgVers:   "v0.48.0",
			registry:  UpgradeRegistry{"v0.34.0": noop},
			wantNeeds: false,
		},
		{
			name:      "config at schema version",
			buildVers: "v0.55.0",
			cfgVers:   "v0.54.0",
			registry:  UpgradeRegistry{"v0.34.0": noop, "v0.54.0": noop},
			wantNeeds: false,
		},
		{
			name:      "config below schema version",
			buildVers: "v0.54.0",
			cfgVers:   "v0.53.0",
			registry:  UpgradeRegistry{"v0.34.0": noop, "v0.54.0": noop},
			wantNeeds: true,
		},
		{
			// Inflated version between registered upgrades: only the
			// later upgrade is outstanding.
			name:      "inflated version below newest upgrade",
			buildVers: "v0.55.0",
			cfgVers:   "v0.48.0",
			registry:  UpgradeRegistry{"v0.34.0": noop, "v0.54.0": noop},
			wantNeeds: true,
		},
		{
			name:      "empty registry",
			buildVers: "v0.55.0",
			cfgVers:   "v0.48.0",
			registry:  UpgradeRegistry{},
			wantNeeds: false,
		},
		{
			name:            "config newer than build",
			buildVers:       "v0.55.0",
			cfgVers:         "v0.58.0",
			registry:        UpgradeRegistry{"v0.34.0": noop, "v0.54.0": noop},
			wantNeeds:       false,
			wantNewerThanBd: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setBuildVersion(t, tc.buildVers)
			_, cfgPath := writeTestConfig(t, "config.version: "+tc.cfgVers+"\n")
			ctx := lg.NewContext(context.Background(), lgt.New(t))
			store := &Store{Path: cfgPath, UpgradeRegistry: tc.registry}

			needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx)
			if tc.wantNewerThanBd {
				require.ErrorIs(t, err, errConfigVersionNewerThanBuild)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.wantNeeds, needsUpgrade)
			require.Equal(t, tc.cfgVers, foundVers)
		})
	}
}

// Test_Store_Load_SchemaCurrent_NoRewrite verifies that loading a
// config whose version is at or above the highest registered upgrade
// version does not rewrite the config file at all, even when the
// build version is newer. The load-save cycle is not byte-preserving
// (unknown keys and YAML comments are dropped on re-marshal), so a
// release without schema changes must leave the file untouched.
func Test_Store_Load_SchemaCurrent_NoRewrite(t *testing.T) {
	setBuildVersion(t, "v0.48.0")

	// The config carries a YAML comment and a key unknown to this
	// build's config.Config struct: both would be destroyed by any
	// load-save rewrite.
	const cfgContent = `config.version: v0.47.0
# A comment that a re-marshal would destroy.
future_field: some value unknown to this build
`
	cfgDir, cfgPath := writeTestConfig(t, cfgContent)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	noop := func(_ context.Context, before []byte) ([]byte, error) {
		return before, nil
	}
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		// The registered upgrade is below the config version: nothing
		// outstanding. This mirrors production for any release that
		// doesn't change the config schema.
		UpgradeRegistry: UpgradeRegistry{"v0.34.0": noop},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.47.0", cfg.Version,
		"config.version must not be inflated to the build version")

	gotContent, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(gotContent),
		"config file must be byte-identical after load")
	requireNoBackupFiles(t, cfgDir)
}

// Test_Store_Load_UpgradeStampsSchemaVersion verifies that when an
// upgrade does run, config.version is stamped with the highest
// registered upgrade version (the schema version), not the build
// version, and that the pre-upgrade backup is written.
func Test_Store_Load_UpgradeStampsSchemaVersion(t *testing.T) {
	setBuildVersion(t, "v0.55.0")

	const cfgContent = "config.version: v0.48.0\n"
	_, cfgPath := writeTestConfig(t, cfgContent)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	var ran34, ran54 bool
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{
			"v0.34.0": func(_ context.Context, before []byte) ([]byte, error) {
				ran34 = true
				return before, nil
			},
			"v0.54.0": func(_ context.Context, before []byte) ([]byte, error) {
				ran54 = true
				return before, nil
			},
		},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.False(t, ran34, "v0.34.0 upgrade is below the config version; must not run")
	require.True(t, ran54, "v0.54.0 upgrade must run")
	require.Equal(t, "v0.54.0", cfg.Version,
		"config.version must be stamped with the schema version, not the build version")

	gotVers, err := LoadVersionFromFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, "v0.54.0", gotVers)

	backupContent, err := os.ReadFile(backupFilePath(cfgPath, "v0.48.0"))
	require.NoError(t, err, "pre-upgrade backup must exist")
	require.Equal(t, cfgContent, string(backupContent))
}

// Test_Store_Save_NewerConfig_BackupBeforeSave verifies the downgrade
// guard: when the loaded config's version is newer than the build
// version (the config was written by a newer sq version), the first
// mutating Save writes a verbatim backup of the on-disk config before
// overwriting it, because the re-marshal through this build's
// config.Config struct drops fields unknown to it.
func Test_Store_Save_NewerConfig_BackupBeforeSave(t *testing.T) {
	setBuildVersion(t, "v0.48.0")

	const cfgContent = `config.version: v0.58.0
# Comment from a newer sq version.
future_field: data this build doesn't understand
`
	cfgDir, cfgPath := writeTestConfig(t, cfgContent)

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.58.0", cfg.Version)

	// Load alone must not rewrite or back up.
	gotContent, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(gotContent))
	requireNoBackupFiles(t, cfgDir)

	// First Save: backup written, then config overwritten.
	require.NoError(t, store.Save(ctx, cfg))

	backupPath := backupFilePath(cfgPath, "v0.58.0")
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err, "backup must exist after mutating Save")
	require.Equal(t, cfgContent, string(backupContent),
		"backup must hold the verbatim pre-save config")

	gotContent, err = os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.NotEqual(t, cfgContent, string(gotContent),
		"config file was re-marshaled by Save")

	// Second Save: the existing backup must not be overwritten with
	// the already-degraded config.
	require.NoError(t, store.Save(ctx, cfg))
	backupContent, err = os.ReadFile(backupPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(backupContent),
		"backup must not be overwritten by subsequent saves")
}

// Test_Store_backupNewerConfig_StatError verifies that a stat error
// on the backup path other than fs.ErrNotExist (e.g. a permission
// error) fails the backup, rather than being treated as a missing
// backup file.
func Test_Store_backupNewerConfig_StatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dir permissions are not enforced this way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("dir permissions are not enforced for root")
	}

	cfgDir, cfgPath := writeTestConfig(t, "config.version: v0.58.0\n")

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, newerCfgVers: "v0.58.0"}

	// Remove the execute bit from the config dir, so that stat on the
	// backup path fails with a permission error, not fs.ErrNotExist.
	require.NoError(t, os.Chmod(cfgDir, 0o600))
	t.Cleanup(func() {
		// Restore perms so t.TempDir cleanup can remove the dir.
		require.NoError(t, os.Chmod(cfgDir, 0o755))
	})

	err := store.backupNewerConfig(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, fs.ErrPermission)
	require.Equal(t, "v0.58.0", store.newerCfgVers,
		"newerCfgVers must not be cleared on failure, so a retry can still back up")
}

// Test_Store_Save_NewerConfig_Prerelease_NoBackup verifies that
// prerelease builds, which are exempt from the newer-than-build
// check, do not write a backup on Save.
func Test_Store_Save_NewerConfig_Prerelease_NoBackup(t *testing.T) {
	setBuildVersion(t, "v0.48.0-dev")

	cfgDir, cfgPath := writeTestConfig(t, "config.version: v0.58.0\n")

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.NoError(t, store.Save(ctx, cfg))
	requireNoBackupFiles(t, cfgDir)
}

// Test_Store_Save_InflatedVersion_NoBackup verifies the compatibility
// case: a config stamped with an inflated (binary) version by an
// older sq release, where the version exceeds this build's highest
// registered upgrade but not the build version, must not trigger the
// downgrade guard.
func Test_Store_Save_InflatedVersion_NoBackup(t *testing.T) {
	setBuildVersion(t, "v0.55.0")

	cfgDir, cfgPath := writeTestConfig(t, "config.version: v0.48.0\n")

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	noop := func(_ context.Context, before []byte) ([]byte, error) {
		return before, nil
	}
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{"v0.34.0": noop},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.NoError(t, store.Save(ctx, cfg))
	requireNoBackupFiles(t, cfgDir)
}

// Test_Load_FreshConfig_StampsSchemaVersion verifies that when no
// config file exists, the fresh config is stamped with the schema
// version (the highest registered upgrade version), not the build
// version. With an empty registry, the config.New default (the build
// version) is retained.
func Test_Load_FreshConfig_StampsSchemaVersion(t *testing.T) {
	noop := func(_ context.Context, before []byte) ([]byte, error) {
		return before, nil
	}

	testCases := []struct {
		name     string
		registry UpgradeRegistry
		wantVers string
	}{
		{
			name:     "registry with upgrades",
			registry: UpgradeRegistry{"v0.34.0": noop, "v0.54.0": noop},
			wantVers: "v0.54.0",
		},
		{
			name:     "empty registry",
			registry: UpgradeRegistry{},
			wantVers: "v0.99.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setBuildVersion(t, "v0.99.0")
			t.Setenv(config.EnvarConfig, t.TempDir())
			ctx := lg.NewContext(context.Background(), lgt.New(t))

			cfg, _, err := Load(ctx, nil, &options.Registry{}, tc.registry)
			require.NoError(t, err)
			require.Equal(t, tc.wantVers, cfg.Version)
		})
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
