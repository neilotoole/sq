package v0_54_0_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0" //nolint:revive
	v0_54_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.54.0" //nolint:revive
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

// TestUpgrade verifies the redact → secrets.reveal rename and polarity
// flip across both the top-level options block and per-source options.
func TestUpgrade(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	const (
		prevVers = "v0.53.0"
		nextVers = "v0.54.0"
	)

	testh.SetBuildVersion(t, nextVers)

	cfgDir := tu.DirCopy(t, "testdata")
	t.Setenv(config.EnvarConfig, cfgDir)

	cfgFilePath := filepath.Join(cfgDir, "sq.yml")

	gotPrevVers, err := yamlstore.LoadVersionFromFile(cfgFilePath)
	require.NoError(t, err)
	require.Equal(t, prevVers, gotPrevVers)

	upgrades := yamlstore.UpgradeRegistry{
		v0_34_0.Version: v0_34_0.Upgrade,
		v0_54_0.Version: v0_54_0.Upgrade,
	}

	optsReg := &options.Registry{}
	cli.RegisterDefaultOpts(optsReg)

	cfg, cfgStore, err := yamlstore.Load(ctx, nil, optsReg, upgrades)
	require.NoError(t, err)

	require.Equal(t, cfgDir, cfgStore.Location())
	require.Equal(t, nextVers, cfg.Version)

	// Top-level: redact: false → secrets.reveal: true.
	require.True(t, secret.OptSecretsReveal.Get(cfg.Options),
		"top-level redact:false should migrate to secrets.reveal:true")
	require.NotContains(t, cfg.Options, "redact",
		"the legacy 'redact' key should be removed from top-level options")

	// Per-source: redact: true (default) is dropped (no key written),
	// since secrets.reveal defaults to false which is the same semantic.
	csvSrc := cfg.Collection.Sources()[1]
	require.Equal(t, true, csvSrc.Options[driver.OptIngestHeader.Key()],
		"unrelated source options should survive the upgrade")
	require.NotContains(t, csvSrc.Options, "redact",
		"per-source 'redact' should be dropped on upgrade")
	require.NotContains(t, csvSrc.Options, "secrets.reveal",
		"per-source redact:true (default) should drop, not write secrets.reveal:false")

	// Per-source: redact: false → secrets.reveal: true on the source.
	xlsxSrc := cfg.Collection.Sources()[2]
	require.Equal(t, true, xlsxSrc.Options[secret.OptSecretsReveal.Key()],
		"per-source redact:false should migrate to secrets.reveal:true")
	require.NotContains(t, xlsxSrc.Options, "redact",
		"per-source 'redact' should be removed on upgrade")

	wantCfgRaw, err := os.ReadFile(filepath.Join("testdata", "want.sq.yml"))
	require.NoError(t, err)

	gotCfgRaw, err := os.ReadFile(filepath.Join(cfgDir, "sq.yml"))
	require.NoError(t, err)

	t.Logf("Output written to: %s", filepath.Join(cfgDir, "sq.yml"))

	wantLines := strings.Split(strings.TrimSpace(string(wantCfgRaw)), "\n")
	gotLines := strings.Split(strings.TrimSpace(string(gotCfgRaw)), "\n")
	require.Equal(t, wantLines, gotLines)
}

// TestUpgrade_NoRedactKey_IsIdempotent verifies that running Upgrade
// against a config that has no "redact" key (whether already-migrated
// or never had one) leaves the user-visible options untouched. Only
// the config.version stamp changes.
func TestUpgrade_NoRedactKey_IsIdempotent(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options:
  format: json
  secrets.reveal: true
collection:
  active.source: "@prod/pg"
  sources:
  - handle: "@prod/pg"
    driver: postgres
    location: postgres://sakila:p_ssW0rd@localhost/sakila
`)

	out, err := v0_54_0.Upgrade(ctx, in)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, ioz.UnmarshallYAML(out, &m))

	require.Equal(t, v0_54_0.Version, m["config.version"],
		"config.version is stamped")

	opts, ok := m["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, opts["secrets.reveal"],
		"existing secrets.reveal:true must survive untouched")
	require.NotContains(t, opts, "redact",
		"no spurious 'redact' key should appear")
}

// TestUpgrade_NonBoolRedact_DropsAndWarns verifies that a corrupt
// (non-bool) value for "redact" is dropped without erroring out.
// The user-visible effect is that secrets.reveal falls back to its
// default; the runtime logs a warning at upgrade time so a
// post-upgrade redaction-behavior change can be traced.
func TestUpgrade_NonBoolRedact_DropsAndWarns(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options:
  format: json
  redact: "true"
collection:
  active.source: "@prod/pg"
  sources:
  - handle: "@prod/pg"
    driver: postgres
    location: postgres://alice:p_ssW0rd@localhost/sakila
    options:
      redact: 42
`)

	out, err := v0_54_0.Upgrade(ctx, in)
	require.NoError(t, err, "upgrade should not error on a non-bool redact value")

	var m map[string]any
	require.NoError(t, ioz.UnmarshallYAML(out, &m))

	opts := m["options"].(map[string]any)
	require.NotContains(t, opts, "redact",
		"top-level non-bool 'redact' must be dropped")
	require.NotContains(t, opts, "secrets.reveal",
		"non-bool 'redact' must fall back to default, not write secrets.reveal")

	srcOpts := m["collection"].(map[string]any)["sources"].([]any)[0].(map[string]any)["options"].(map[string]any)
	require.NotContains(t, srcOpts, "redact",
		"per-source non-bool 'redact' must be dropped")
	require.NotContains(t, srcOpts, "secrets.reveal")
}

// TestUpgrade_CorruptOptionsShape verifies that a present-but-wrong-
// shape "options" entry surfaces as an error rather than silently
// stamping config.version and skipping the redact translation.
func TestUpgrade_CorruptOptionsShape(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	in := []byte(`config.version: v0.53.0
options: "not a map"
`)

	_, err := v0_54_0.Upgrade(ctx, in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "corrupt config")
}
