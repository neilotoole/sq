package v0_34_0_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/config/yamlstore"
	v0_34_0 "github.com/neilotoole/sq/cli/config/yamlstore/upgrades/v0.34.0"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

func TestUpgrade(t *testing.T) {
	log := lgt.New(t)
	ctx := lg.NewContext(context.Background(), log)

	const (
		prevVers   = "v0.33.0"
		nextVers   = "v0.34.0"
		handlePg   = "@prod/pg"
		handleCSV  = "@csv"
		handleXLSX = "@xlsx"
	)

	testh.SetBuildVersion(t, nextVers)

	// The sq.yml file in cfgDir is on v0.33.0
	cfgDir := tu.DirCopy(t, "testdata", true)
	t.Setenv(config.EnvarConfig, cfgDir)

	cfgFilePath := filepath.Join(cfgDir, "sq.yml")

	gotPrevVers, err := yamlstore.LoadVersionFromFile(cfgFilePath)
	require.NoError(t, err)
	require.Equal(t, prevVers, gotPrevVers)

	upgrades := yamlstore.UpgradeRegistry{
		v0_34_0.Version: v0_34_0.Upgrade,
	}

	optsReg := &options.Registry{}
	cli.RegisterDefaultOpts(optsReg)

	cfg, cfgStore, err := yamlstore.Load(ctx, nil, optsReg, upgrades)
	require.NoError(t, err)

	require.Equal(t, cfgDir, cfgStore.Location())
	require.Equal(t, nextVers, cfg.Version)
	require.Equal(t, format.JSON, cli.OptFormat.Get(cfg.Options))
	require.Equal(t, time.Second*100, cli.OptPingCmdTimeout.Get(cfg.Options))
	require.Len(t, cfg.Collection.Sources(), 3)
	src0 := cfg.Collection.Sources()[0]
	require.Equal(t, handlePg, src0.Handle)
	require.Equal(t, postgres.Type, src0.Type)
	require.Equal(t, "prod", cfg.Collection.ActiveGroup())
	require.NotNil(t, cfg.Collection.Active())
	require.Equal(t, handlePg, cfg.Collection.Active().Handle)

	src1 := cfg.Collection.Sources()[1]
	require.Equal(t, handleCSV, src1.Handle)
	require.Equal(t, csv.TypeCSV, src1.Type)
	require.Equal(t, true, src1.Options[driver.OptIngestHeader.Key()])

	src2 := cfg.Collection.Sources()[2]
	require.Equal(t, handleXLSX, src2.Handle)
	require.Equal(t, xlsx.Type, src2.Type)
	require.Equal(t, false, src2.Options[driver.OptIngestHeader.Key()])

	wantCfgRaw, err := os.ReadFile(filepath.Join("testdata", "want.sq.yml"))
	require.NoError(t, err)

	gotCfgRaw, err := os.ReadFile(filepath.Join(cfgDir, "sq.yml"))
	require.NoError(t, err)

	t.Logf("Output written to: %s", filepath.Join(cfgDir, "sq.yml"))

	require.Equal(t, strings.TrimSpace(string(wantCfgRaw)), strings.TrimSpace(string(gotCfgRaw)))
}
