package cli_test

import (
	"testing"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdRemove(t *testing.T) {
	th := testh.New(t)

	// 1. Should fail if bad handle
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("rm", "@not_a_source")
	require.Error(t, err)

	// 2. Check normal operation
	src := th.Source(sakila.SL3)
	tr = testrun.New(th.Context, t, nil).Add(*src)

	// The src we just added should be the active src
	activeSrc := tr.Run.Config.Collection.Active()
	require.NotNil(t, activeSrc)
	require.Equal(t, src.Handle, activeSrc.Handle)

	err = tr.Exec("rm", src.Handle)
	require.NoError(t, err)

	activeSrc = tr.Run.Config.Collection.Active()
	require.Nil(t, activeSrc, "should be no active src anymore")
}
