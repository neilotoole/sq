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
