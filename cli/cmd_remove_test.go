package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdRemove(t *testing.T) {
	th := testh.New(t)

	// 1. Should fail if bad handle
	ru := newRun(t)
	err := ru.Exec("rm", "@not_a_source")
	require.Error(t, err)

	// 2. Check normal operation
	src := th.Source(sakila.SL3)
	ru = newRun(t).add(*src)

	// The src we just added should be the active src
	activeSrc := ru.rc.Config.Sources.Active()
	require.NotNil(t, activeSrc)
	require.Equal(t, src.Handle, activeSrc.Handle)

	err = ru.Exec("rm", src.Handle)
	require.NoError(t, err)

	activeSrc = ru.rc.Config.Sources.Active()
	require.Nil(t, activeSrc, "should be no active src anymore")
}
