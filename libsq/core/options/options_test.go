package options_test

import (
	"os"
	"testing"
	"time"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/output/format"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/stretchr/testify/require"
)

type config struct {
	Options options.Options `yaml:"options"`
}

func TestOptions(t *testing.T) {
	log := slogt.New(t)
	b, err := os.ReadFile("testdata/good.01.yml")
	require.NoError(t, err)

	reg := options.DefaultRegistry
	log.Debug("DefaultRegistry", "reg", reg)

	cfg := &config{Options: options.Options{}}
	require.NoError(t, ioz.UnmarshallYAML(b, cfg))
	cfg.Options, err = options.DefaultRegistry.Process(cfg.Options)
	require.NoError(t, err)

	require.Equal(t, format.CSV, cli.OptOutputFormat.Get(cfg.Options))
	require.Equal(t, true, cli.OptPrintHeader.Get(cfg.Options))
	require.Equal(t, time.Second*10, cli.OptPingTimeout.Get(cfg.Options))
	require.Equal(t, time.Millisecond*500, cli.OptShellCompletionTimeout.Get(cfg.Options))

	require.Equal(t, 50, driver.OptConnMaxOpen.Get(cfg.Options))
	require.Equal(t, 100, driver.OptConnMaxIdle.Get(cfg.Options))
	require.Equal(t, time.Second*100, driver.OptConnMaxIdleTime.Get(cfg.Options))
	require.Equal(t, time.Minute*5, driver.OptConnMaxLifetime.Get(cfg.Options))
}
