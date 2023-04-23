package options_test

import (
	"os"
	"testing"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/ryboe/q"
	"github.com/stretchr/testify/require"

	// Import CLI for side effect of loading opts.
	_ "github.com/neilotoole/sq/cli"
)

type config struct {
	Options options.Options `yaml:"options"`
}

func TestSmoke(t *testing.T) {
	cfg := &config{Options: options.Options{}}

	b, err := os.ReadFile("testdata/good.01.yml")
	require.NoError(t, err)

	require.NoError(t, ioz.UnmarshallYAML(b, cfg))

	q.Q(cfg)

	cfg.Options, err = options.DefaultRegistry.Process(cfg.Options)
	require.NoError(t, err)
	q.Q(cfg)
}

func getOpts(t *testing.T) options.Options {
	cfg := &config{Options: options.Options{}}

	b, err := os.ReadFile("testdata/good.01.yml")
	require.NoError(t, err)

	require.NoError(t, ioz.UnmarshallYAML(b, cfg))

	opts, err := options.DefaultRegistry.Process(cfg.Options)
	require.NoError(t, err)

	return opts
}

func Test2(t *testing.T) {
	opts := getOpts(t)

	d := driver.ConnMaxLifetime.Get(opts)
	t.Logf("%s: %v", driver.ConnMaxLifetime.Key(), d)
}
