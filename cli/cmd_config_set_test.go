package cli_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// TestCmdConfigSet verifies that setting config options actually takes effect.
// In this test, we use driver.OptResultColRename, setting that template such
// that the column name is transformed to uppercase.
func TestCmdConfigSet(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)

	tr := testrun.New(th.Context, t, nil).Hush().Add(*src)
	err := tr.Exec(".actor | .[0]")
	require.NoError(t, err)

	got := tr.Out.String()
	require.Contains(t, got, "actor_id")
	require.NotContains(t, got, "ACTOR_ID")

	tr = testrun.New(th.Context, t, tr)
	const tpl = `{{.Name | upper}}`
	err = tr.Exec("config", "set", driver.OptResultColRename.Key(), stringz.ShellEscape(tpl))
	require.NoError(t, err)

	tr = testrun.New(th.Context, t, tr)
	err = tr.Exec(".actor | .[0]")
	require.NoError(t, err)
	got = tr.Out.String()
	require.Contains(t, got, "ACTOR_ID")
	require.NotContains(t, got, "actor_id")
}
