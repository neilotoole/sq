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
	needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx, store.UpgradeRegistry.highestVersion())
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
	needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx, store.UpgradeRegistry.highestVersion())
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
		{
			// Misbuilt binary: a registered upgrade key (v0.55.0) exceeds
			// the build version (v0.50.0), and the config (v0.52.0) sits
			// between them. The schema axis alone would say needsUpgrade
			// (0.52 < 0.55), but the config is newer than the build, so
			// the upgrade must be suppressed (wantNeeds=false) and the
			// newer-than-build error returned. This exercises the explicit
			// override in checkNeedsUpgrade.
			name:            "registry key above build version",
			buildVers:       "v0.50.0",
			cfgVers:         "v0.52.0",
			registry:        UpgradeRegistry{"v0.34.0": noop, "v0.55.0": noop},
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

			needsUpgrade, foundVers, err := store.checkNeedsUpgrade(ctx, store.UpgradeRegistry.highestVersion())
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

// Test_Store_Load_Upgrade_PreservesGuardBackup verifies that the
// pre-upgrade backup is written at most once: when a backup for the
// same version already exists (here, the downgrade guard's pristine
// copy of a newer config, written by Store.backupNewerConfig in an
// older binary), the upgrade must not overwrite it with the degraded
// on-disk config.
func Test_Store_Load_Upgrade_PreservesGuardBackup(t *testing.T) {
	setBuildVersion(t, "v0.56.0")

	// The on-disk config: stamped v0.55.0, already degraded by an
	// older (downgraded) binary's re-marshaling save.
	const degraded = "config.version: v0.55.0\n"
	_, cfgPath := writeTestConfig(t, degraded)

	// The guard backup: the pristine v0.55-written config that the
	// downgrade guard preserved before the older binary's first save.
	const pristine = `config.version: v0.55.0
# Comment from the newer sq version.
future_field: data the older build didn't understand
`
	backupPath := backupFilePath(cfgPath, "v0.55.0")
	require.NoError(t, os.WriteFile(backupPath, []byte(pristine), 0o644))

	ctx := lg.NewContext(context.Background(), lgt.New(t))

	var ran bool
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{
			"v0.56.0": func(_ context.Context, before []byte) ([]byte, error) {
				ran = true
				return before, nil
			},
		},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.True(t, ran, "v0.56.0 upgrade must run")
	require.Equal(t, "v0.56.0", cfg.Version)

	got, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	require.Equal(t, pristine, string(got),
		"existing guard backup must not be clobbered by the pre-upgrade backup")
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

// Test_Store_Load_ClearsNewerCfgVers_OnReload verifies that the sticky
// newer-than-build state is cleared when the same Store later loads a
// config that is no longer newer than the build (e.g. another process
// normalized it while we held it). Otherwise a stale newerCfgVers would
// trigger a spurious backup on the next Save.
func Test_Store_Load_ClearsNewerCfgVers_OnReload(t *testing.T) {
	setBuildVersion(t, "v0.48.0")

	const newerContent = `config.version: v0.58.0
future_field: data this build doesn't understand
`
	cfgDir, cfgPath := writeTestConfig(t, newerContent)
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{},
	}

	// First load: config is newer than the build, so newerCfgVers is set.
	_, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.58.0", store.newerCfgVers,
		"a newer-than-build config must set newerCfgVers")

	// The config on disk is replaced with a current one and reloaded on
	// the same Store; the sticky flag must be cleared.
	require.NoError(t, os.WriteFile(cfgPath, []byte("config.version: v0.48.0\n"), 0o600))
	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Empty(t, store.newerCfgVers,
		"reloading a non-newer config must clear the sticky newerCfgVers")

	// A subsequent Save must not write a backup: the config is no longer
	// newer than the build.
	require.NoError(t, store.Save(ctx, cfg))
	requireNoBackupFiles(t, cfgDir)
}

// Test_Store_backupNewerConfig_NonRegularBackupPath verifies that when
// the backup path exists but is not a regular file (e.g. a directory),
// Save fails and does not overwrite the config, rather than treating the
// non-regular path as a valid backup and silently proceeding.
func Test_Store_backupNewerConfig_NonRegularBackupPath(t *testing.T) {
	setBuildVersion(t, "v0.48.0")

	const cfgContent = "config.version: v0.58.0\nfuture_field: keep\n"
	_, cfgPath := writeTestConfig(t, cfgContent)
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)

	// Put a directory where the backup file would go: it can't serve as
	// a backup, and Save must refuse rather than skip the backup.
	require.NoError(t, os.Mkdir(backupFilePath(cfgPath, "v0.58.0"), 0o700))

	err = store.Save(ctx, cfg)
	require.Error(t, err, "Save must fail when the backup path is not a regular file")

	got, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(got),
		"config must be left intact when the backup couldn't be written")
}

// Test_Store_Load_MultiStepUpgrade_Chains verifies that a Load spanning
// two registered upgrade funcs runs them in ascending version order,
// threads each func's output into the next, stamps the highest version,
// and persists a re-loadable config (the canonicalization the removed
// outer load-save cycle used to provide is done inside doUpgrade).
func Test_Store_Load_MultiStepUpgrade_Chains(t *testing.T) {
	setBuildVersion(t, "v0.55.0")

	var order []string
	var step2Input string
	step1 := func(_ context.Context, before []byte) ([]byte, error) {
		order = append(order, "v0.40.0")
		return append(before, []byte("# step1\n")...), nil
	}
	step2 := func(_ context.Context, before []byte) ([]byte, error) {
		order = append(order, "v0.50.0")
		step2Input = string(before)
		return append(before, []byte("# step2\n")...), nil
	}

	_, cfgPath := writeTestConfig(t, "config.version: v0.30.0\n")
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{"v0.40.0": step1, "v0.50.0": step2},
	}

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"v0.40.0", "v0.50.0"}, order,
		"both upgrade funcs must run, in ascending version order")
	require.Contains(t, step2Input, "# step1",
		"each func must receive the previous func's output")
	require.Equal(t, "v0.50.0", cfg.Version,
		"config.version must be stamped to the highest registered upgrade")

	// The on-disk config must be re-loadable and stamped, and a second
	// load must not re-run the upgrades.
	reloaded, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.50.0", reloaded.Version)
	require.Len(t, order, 2, "a second load must not re-run the upgrade funcs")
}

// Test_LoadVersionFromFile covers the version-field parsing edge cases:
// a malformed or missing config.version is a load-time error a real
// (hand-edited or corrupted) config can hit.
func Test_LoadVersionFromFile(t *testing.T) {
	testCases := []struct {
		name    string
		content string
		want    string
		wantErr string // error-message substring; "" means expect success
	}{
		{"valid", "config.version: v0.34.0\n", "v0.34.0", ""},
		{"legacy version field", "version: v0.34.0\n", "v0.34.0", ""},
		{"legacy config_version field", "config_version: v0.34.0\n", "v0.34.0", ""},
		{"missing field", "options:\n  format: json\n", "", "does not have a version field"},
		{"empty string value", "config.version: \"\"\n", "", "does not have a version field"},
		{"non-string value", "config.version: 123\n", "", "invalid value for"},
		{"invalid semver", "config.version: not-a-version\n", "", "invalid semver value for"},
		{"malformed yaml", "key: [unclosed\n", "", "unmarshal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfgPath := writeTestConfig(t, tc.content)
			got, err := LoadVersionFromFile(cfgPath)
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// Test_checkNeedsUpgrade_belowMinVersion verifies that a config version
// below MinConfigVersion is a fatal error, not silently upgraded.
func Test_checkNeedsUpgrade_belowMinVersion(t *testing.T) {
	setBuildVersion(t, "v0.55.0")
	noop := func(_ context.Context, before []byte) ([]byte, error) { return before, nil }

	// v0.0.0-aaa sorts below MinConfigVersion (v0.0.0-dev) by prerelease.
	_, cfgPath := writeTestConfig(t, "config.version: v0.0.0-aaa\n")
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, UpgradeRegistry: UpgradeRegistry{"v0.34.0": noop}}

	_, _, err := store.checkNeedsUpgrade(ctx, store.UpgradeRegistry.highestVersion())
	require.Error(t, err)
	require.Contains(t, err.Error(), "less than minimum")
}

// Test_Store_doUpgrade_DirectCalls covers doUpgrade's two guard branches
// reached only by a direct caller: an invalid target version, and a
// target with no upgrade funcs in range (config left untouched).
func Test_Store_doUpgrade_DirectCalls(t *testing.T) {
	noop := func(_ context.Context, before []byte) ([]byte, error) { return before, nil }
	ctx := lg.NewContext(context.Background(), lgt.New(t))

	const cfgContent = "config.version: v0.34.0\n"
	_, cfgPath := writeTestConfig(t, cfgContent)
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{"v0.34.0": noop},
	}

	// Invalid target version is rejected before any file change.
	_, err := store.doUpgrade(ctx, "v0.34.0", "not-a-semver")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid semver")

	// No upgrade funcs in range: the config is loaded and returned
	// untouched, with no error.
	cfg, err := store.doUpgrade(ctx, "v0.34.0", "v0.34.0")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	got, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(got), "no-op upgrade must not rewrite the config")
}

// Test_Store_Load_UpgradeFuncError verifies that a failing upgrade func
// surfaces as a load error and leaves the config file unchanged (the
// upgraded bytes are written only after every func succeeds).
func Test_Store_Load_UpgradeFuncError(t *testing.T) {
	setBuildVersion(t, "v0.55.0")
	boom := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, fs.ErrInvalid // any non-nil error
	}

	const cfgContent = "config.version: v0.30.0\n"
	cfgDir, cfgPath := writeTestConfig(t, cfgContent)
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{"v0.50.0": boom},
	}

	_, err := store.Load(ctx)
	require.Error(t, err, "a failing upgrade func must fail the load")

	// The original config must be intact (only a verbatim backup may
	// have been written before the func ran).
	got, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, cfgContent, string(got),
		"a failed upgrade must not corrupt the config")
	_ = cfgDir
}

// Test_Store_applyVersionCheck_FatalClearsNewerCfgVers verifies that a
// fatal (non-newer-than-build) check error clears newerCfgVers, so a
// reused Store can't carry stale state into a later Save's backup.
func Test_Store_applyVersionCheck_FatalClearsNewerCfgVers(t *testing.T) {
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: "irrelevant.yml", newerCfgVers: "v0.58.0"}

	// fs.ErrNotExist stands in for any fatal, non-newer check error.
	err := store.applyVersionCheck(ctx, "v0.30.0", fs.ErrNotExist)
	require.Error(t, err)
	require.Empty(t, store.newerCfgVers,
		"a fatal version-check error must clear newerCfgVers")
}

// Test_Store_backupNewerConfig_ConfigDeleted verifies that if the config
// file is gone by the time Save runs, backupNewerConfig treats it as
// nothing-to-back-up: it clears the flag and returns nil rather than
// failing.
func Test_Store_backupNewerConfig_ConfigDeleted(t *testing.T) {
	_, cfgPath := writeTestConfig(t, "config.version: v0.58.0\n")
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, newerCfgVers: "v0.58.0"}

	require.NoError(t, os.Remove(cfgPath))

	require.NoError(t, store.backupNewerConfig(ctx))
	require.Empty(t, store.newerCfgVers,
		"flag must be cleared when there is nothing on disk to back up")
}

// Test_Store_doLoad_EmptyVersionStampsBuildVersion verifies the fallback
// stamping for a config with no version field, loaded via a registry-less
// Store (the only path that reaches doLoad with an empty version, since a
// registry makes the missing version fatal in checkNeedsUpgrade).
func Test_Store_doLoad_EmptyVersionStampsBuildVersion(t *testing.T) {
	setBuildVersion(t, "v0.55.0")
	_, cfgPath := writeTestConfig(t, "options:\n  format: json\n")
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{Path: cfgPath, OptionsRegistry: &options.Registry{}} // UpgradeRegistry nil

	cfg, err := store.Load(ctx)
	require.NoError(t, err)
	require.Equal(t, "v0.55.0", cfg.Version,
		"an empty version stamps the build version when no registry is configured")
}

// Test_Store_writeConfigBackupOnce_StatError verifies that a stat failure
// other than ErrNotExist (here ENOTDIR, via a non-directory in the path)
// aborts rather than silently proceeding without a backup.
func Test_Store_writeConfigBackupOnce_StatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ENOTDIR path semantics differ on Windows")
	}
	tmp := t.TempDir()
	notDir := filepath.Join(tmp, "notdir")
	require.NoError(t, os.WriteFile(notDir, []byte("x"), 0o600))

	ctx := lg.NewContext(context.Background(), lgt.New(t))
	// fs.Path sits under a regular file, so stat on the backup path
	// fails with ENOTDIR (not ErrNotExist).
	store := &Store{Path: filepath.Join(notDir, "sq.yml")}

	_, err := store.writeConfigBackupOnce(ctx, "v0.58.0", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to stat config backup")
}

// Test_Store_Load_RejectsBrokenConfig verifies that a broken config fails
// Load with a clear error rather than panicking or silently loading a
// degraded config. Broken config loading would be fatal, so each case
// must surface a diagnostic error.
func Test_Store_Load_RejectsBrokenConfig(t *testing.T) {
	setBuildVersion(t, "v0.55.0")
	noop := func(_ context.Context, b []byte) ([]byte, error) { return b, nil }

	testCases := []struct {
		name     string
		content  string
		registry UpgradeRegistry
		wantErr  string
	}{
		{
			name:     "malformed version",
			content:  "config.version: not-a-version\n",
			registry: UpgradeRegistry{"v0.34.0": noop},
			wantErr:  "invalid semver",
		},
		{
			name:     "missing version",
			content:  "options:\n  format: json\n",
			registry: UpgradeRegistry{"v0.34.0": noop},
			wantErr:  "does not have a version field",
		},
		{
			name:     "below minimum version",
			content:  "config.version: v0.0.0-aaa\n",
			registry: UpgradeRegistry{"v0.34.0": noop},
			wantErr:  "less than minimum",
		},
		{
			// Registry-less so checkNeedsUpgrade is skipped and the parse
			// error surfaces from doLoad's unmarshal instead.
			name:     "malformed yaml",
			content:  "key: [unclosed\n",
			registry: nil,
			wantErr:  "unmarshal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, cfgPath := writeTestConfig(t, tc.content)
			ctx := lg.NewContext(context.Background(), lgt.New(t))
			store := &Store{
				Path:            cfgPath,
				OptionsRegistry: &options.Registry{},
				UpgradeRegistry: tc.registry,
			}

			_, err := store.Load(ctx)
			require.Error(t, err, "broken config must fail load")
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// Test_Store_Load_RepairsDanglingActiveSource verifies doLoad's integrity
// repair path: a config whose active.source references a missing handle is
// repaired (active source unset) and persisted, and a re-load then
// succeeds.
func Test_Store_Load_RepairsDanglingActiveSource(t *testing.T) {
	setBuildVersion(t, "v0.55.0")
	noop := func(_ context.Context, b []byte) ([]byte, error) { return b, nil }

	const cfgContent = `config.version: v0.34.0
collection:
  active.source: '@ghost'
  sources:
    - handle: '@real'
      driver: sqlite3
      location: sqlite3:///tmp/real.db
`
	_, cfgPath := writeTestConfig(t, cfgContent)
	ctx := lg.NewContext(context.Background(), lgt.New(t))
	store := &Store{
		Path:            cfgPath,
		OptionsRegistry: &options.Registry{},
		UpgradeRegistry: UpgradeRegistry{"v0.34.0": noop},
	}

	// First load detects the dangling active source, repairs it, and
	// returns the repair notice as an error.
	_, err := store.Load(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has been repaired")

	// The repair was persisted, so a second load succeeds.
	reloaded, err := store.Load(ctx)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
	require.Empty(t, reloaded.Collection.ActiveHandle(),
		"the dangling active source must have been unset")
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
