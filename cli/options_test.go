package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
)

func TestRegisterDefaultOpts(t *testing.T) {
	log := lgt.New(t)
	reg := &options.Registry{}

	log.Debug("options.Registry (before)", "reg", reg)
	cli.RegisterDefaultOpts(reg)
	log.Debug("options.Registry (after)", "reg", reg)

	keys := reg.Keys()
	require.Len(t, keys, 47)

	for _, opt := range reg.Opts() {
		opt := opt
		t.Run(opt.Key(), func(t *testing.T) {
			require.NotNil(t, opt)
			require.NotEmpty(t, opt.Key())
			require.NotNil(t, opt.GetAny(nil))
			require.NotNil(t, opt.DefaultAny())
			require.Equal(t, opt.GetAny(nil), opt.DefaultAny())
			require.NotEmpty(t, opt.Usage())
			require.True(t, opt.Short() >= 0)
			require.Equal(t, opt.Key(), opt.String())
			require.NotEmpty(t, opt.Help())
		})
	}
}
