package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neilotoole/sq/cli/buildinfo"

	"github.com/neilotoole/sq/libsq/core/ioz"

	"github.com/neilotoole/sq/drivers/postgres"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/config"
)

func setBuildVersion(t testing.TB, vers string) {
	prevVers := buildinfo.Version
	t.Setenv(buildinfo.EnvOverrideVersion, vers)
	buildinfo.Version = vers
	t.Cleanup(func() {
		buildinfo.Version = prevVers
	})
}

func Test_Upgrade_v0_34_0(t *testing.T) {
	const (
		prevVers    = "v0.33.0"
		nextVers    = "v0.34.0"
		testdataDir = "testdata/upgrade/" + nextVers
		handle      = "@prod/sakila"
	)

	setBuildVersion(t, nextVers)

	// The sq.yml file in cfgDir is on v0.33.0
	cfgDir := tutil.DirCopy(t, testdataDir, true)
	t.Setenv(config.EnvarConfig, cfgDir)

	cfgFile := filepath.Join(cfgDir, "sq.yml")

	gotPrevVers, err := config.LoadVersion(cfgFile)
	require.NoError(t, err)
	require.Equal(t, prevVers, gotPrevVers)

	t.Logf("config file (before): %s", cfgFile)
	_ = ioz.FPrintFile(tutil.Writer(t), cfgFile)

	cfg, cfgStore, err := config.DefaultLoad(nil)
	require.NoError(t, err)

	t.Logf("config file (after): %s", cfgFile)
	_ = ioz.FPrintFile(tutil.Writer(t), cfgFile)

	require.Equal(t, cfgDir, cfgStore.Location())
	require.Equal(t, nextVers, cfg.Version)
	require.Equal(t, config.FormatJSON, cfg.Options.Format)
	require.Equal(t, time.Second*100, cfg.Options.PingTimeout)
	require.Len(t, cfg.Collection.Sources(), 1)
	src0 := cfg.Collection.Sources()[0]
	require.Equal(t, handle, src0.Handle)
	require.Equal(t, postgres.Type, src0.Type)
	require.Equal(t, "prod", cfg.Collection.ActiveGroup())
	require.NotNil(t, cfg.Collection.Active())
	require.Equal(t, handle, cfg.Collection.Active().Handle)

	wantCfgRaw, err := os.ReadFile(filepath.Join(testdataDir, "want.sq.yml"))
	require.NoError(t, err)

	gotCfgRaw, err := os.ReadFile(filepath.Join(cfgDir, "sq.yml"))
	require.NoError(t, err)

	require.Equal(t, strings.TrimSpace(string(wantCfgRaw)), strings.TrimSpace(string(gotCfgRaw)))
}
