package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdExplore_HelpFlag(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("explore", "--help")
	require.NoError(t, err)
	out := tr.Out.String()
	require.Contains(t, out, "Inspect data source schema interactively")
	require.Contains(t, out, "--emit-handle")
	require.Contains(t, out, "--no-tui")
	require.Contains(t, out, "--preview-rows")
}

func TestCmdExplore_NoArg_NoActiveSource(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("explore")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no data source specified")
}

func TestCmdExplore_NoTUI_PrintsOverview(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("explore", "--no-tui", src.Handle)
	require.NoError(t, err)
	require.Contains(t, tr.Out.String(), src.Handle)
}

func TestCmdExplore_ParseTableHandle(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("explore", "--no-tui", src.Handle+".actor")
	require.NoError(t, err, "context-aware table argument should be accepted")
}

func TestCmdExplore_EmitHandle_WithNoTUI(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	tr := testrun.New(th.Context, t, nil).Add(*src)
	err := tr.Exec("explore", "--no-tui", "--emit-handle", src.Handle+".actor")
	require.NoError(t, err)
	require.Contains(t, tr.Out.String(), src.Handle+".actor",
		"the emitted handle should appear in stdout under --no-tui")
}
