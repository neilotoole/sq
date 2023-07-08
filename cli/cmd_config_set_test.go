package cli_test

import (
	"context"
	"testing"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/testh/proj"

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
	const tpl = `{{.Name | upper}}_{{.Alpha}}`
	err = tr.Exec("config", "set", driver.OptResultColRename.Key(), stringz.ShellEscape(tpl))
	require.NoError(t, err)

	tr = testrun.New(th.Context, t, tr)
	err = tr.Exec(".actor | .[0]")
	require.NoError(t, err)
	got = tr.Out.String()
	require.Contains(t, got, "ACTOR_ID_A")
	require.NotContains(t, got, "actor_id")
}

func TestConfigSetBaseOptOnSourceIsError(t *testing.T) {
	const handle = "@actor"

	ctx := context.Background()
	tr := testrun.New(ctx, t, nil).Hush()

	fp := proj.Abs("drivers/csv/testdata/sakila-csv/actor.csv")
	require.NoError(t, tr.Exec("add", fp, "--handle="+handle))

	// Note that driver.OptResultColRename is not a source tag.
	require.False(t, driver.OptResultColRename.HasTag(options.TagSource))

	const tpl = "{{.Name}}"
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("config", "set",
		driver.OptResultColRename.Key(), tpl,
	), "should work for base config")

	tr = testrun.New(ctx, t, tr)
	err := tr.Exec("config", "set",
		"--"+flag.ConfigSrc, handle,
		driver.OptResultColRename.Key(), tpl,
	)
	require.Error(t, err, "should NOT work for source config")
}

// TestSourceOptOverridesBaseOpt tests that source-specific opts override base opts.
func TestSourceOptOverridesBaseOpt(t *testing.T) {
	const handle = "@actor"

	opt := driver.OptIngestColRename
	require.True(t, opt.HasTag(options.TagSource),
		"verify that the opt applies to sources")

	testCases := []struct {
		fp      string
		tblName string
	}{
		{sakila.PathXLSXActorHeader, ".actor"},
		{sakila.PathCSVActor, ".data"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.fp, func(t *testing.T) {
			ctx := context.Background()
			tr := testrun.New(ctx, t, nil).Hush()

			fp := proj.Abs(sakila.PathCSVActor)
			require.NoError(t, tr.Exec("add", fp, "--handle="+handle))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", tc.tblName))
			defaultHeaders := sakila.TblActorCols()
			data := tr.BindCSV()
			require.Equal(t, defaultHeaders, data[0])

			const baseTpl = "base_{{.Name}}"
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				opt.Key(), baseTpl,
			))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", tc.tblName))
			wantBaseHeaders := stringz.PrefixSlice(sakila.TblActorCols(), "base_")
			data = tr.BindCSV()
			require.Equal(t, wantBaseHeaders, data[0])

			const srcTpl = "src_{{.Name}}"
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				"--"+flag.ConfigSrc, handle,
				opt.Key(), srcTpl,
			))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", tc.tblName))
			wantSrcHeaders := stringz.PrefixSlice(sakila.TblActorCols(), "src_")
			data = tr.BindCSV()
			require.Equal(t, wantSrcHeaders, data[0])

			// Unset base opt
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				opt.Key(), "",
			))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", tc.tblName))
			data = tr.BindCSV()
			require.Equal(t, wantSrcHeaders, data[0], "should still show src headers")

			// Unset src opt
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				"--"+flag.ConfigSrc, handle,
				opt.Key(), "",
			))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", tc.tblName))
			data = tr.BindCSV()
			require.Equal(t, defaultHeaders, data[0], "should be back to default headers")
		})
	}
}

// TestColRenameOptsInteraction tests the interaction of
// driver.OptIngestColRename and driver.OptResultColRename.
func TestColRenameOptsInteraction(t *testing.T) {
	testCases := []struct {
		fp      string
		tblName string
	}{
		{sakila.PathXLSXActorHeader, "actor"},
		{sakila.PathCSVActor, "data"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.fp, func(t *testing.T) {
			ctx := context.Background()
			tr := testrun.New(ctx, t, nil)

			fp := proj.Abs(tc.fp)
			err := tr.Exec("add", fp)
			require.NoError(t, err)

			const ingestTpl = "x_{{.Name}}"
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				driver.OptIngestColRename.Key(), ingestTpl,
			))

			const resultTpl = "{{.Name}}_y"
			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("config", "set",
				driver.OptResultColRename.Key(), resultTpl,
			))

			tr = testrun.New(ctx, t, tr)
			require.NoError(t, tr.Exec("--csv", "."+tc.tblName))
			wantHeaders := []string{"x_actor_id_y", "x_first_name_y", "x_last_name_y", "x_last_update_y"}
			data := tr.BindCSV()
			require.Equal(t, wantHeaders, data[0])
		})
	}
}
