package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
)

func TestRegisterDefaultOpts(t *testing.T) {
	reg := &options.Registry{}
	cli.RegisterDefaultOpts(reg)
	lgt.New(t).Debug("options.Registry (after)", "reg", reg)

	keys := reg.Keys()
	require.Len(t, keys, 56)

	for _, opt := range reg.Opts() {
		opt := opt
		t.Run(opt.Key(), func(t *testing.T) {
			require.NotNil(t, opt)
			require.NotEmpty(t, opt.Key())
			require.NotNil(t, opt.GetAny(nil))
			require.NotNil(t, opt.DefaultAny())
			require.Equal(t, opt.GetAny(nil), opt.DefaultAny())
			require.NotEmpty(t, opt.Usage())
			require.NotEmpty(t, opt.Flag().Usage)
			require.True(t, opt.Flag().Short >= 0)
			require.Equal(t, opt.Key(), opt.String())
			require.NotEmpty(t, opt.Help())
		})
	}
}
