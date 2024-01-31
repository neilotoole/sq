package cli_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/cobraz"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// testComplete is a helper for testing cobra completion.
func testComplete(tb testing.TB, from *testrun.TestRun, args ...string) completion {
	tb.Helper()
	var ctx context.Context
	if from == nil {
		ctx = lg.NewContext(context.Background(), lgt.New(tb))
	} else {
		ctx = from.Context
	}

	ctx = enableCompletionLog(ctx)
	tr := testrun.New(ctx, tb, from)
	args = append([]string{cobra.ShellCompRequestCmd}, args...)

	err := tr.Exec(args...)
	require.NoError(tb, err)

	c := parseCompletion(tr)
	return c
}

// parseCompletion parses the output of cobra "__complete".
// Example output:
//
//	@active
//	@sakila
//	:4
//	Completion ended with directive: ShellCompDirectiveNoFileComp
//
// The tr.T test will fail on any error.
func parseCompletion(tr *testrun.TestRun) completion {
	c := completion{
		stdout: tr.Out.String(),
		stderr: tr.ErrOut.String(),
	}

	lines := strings.Split(strings.TrimSpace(c.stdout), "\n")
	require.True(tr.T, len(lines) >= 1)
	c.values = lines[:len(lines)-1]

	result, err := strconv.Atoi(lines[len(lines)-1][1:])
	require.NoError(tr.T, err)
	c.result = cobra.ShellCompDirective(result)

	c.directives = cobraz.ParseDirectivesLine(c.stderr)
	return c
}

// completion models the result returned from the cobra "__complete" command.
type completion struct {
	stdout     string
	stderr     string
	values     []string
	result     cobra.ShellCompDirective
	directives []cobra.ShellCompDirective
}

// TestCompleteFlagActiveSchema_query_cmds tests flag.ActiveSchema
// behavior for the query commands (slq, sql).
//
// See also: TestCompleteFlagActiveSchema_inspect.
func TestCompleteFlagActiveSchema_query_cmds(t *testing.T) { //nolint:tparallel
	const wantDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

	testCases := []struct {
		handles           []string
		arg               string
		withFlagActiveSrc string
		wantContains      []string
		wantDirective     cobra.ShellCompDirective
	}{
		{
			handles:       []string{sakila.Pg},
			arg:           "saki",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "",
			wantContains:  []string{"public", "sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "sakila",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "sakila.pub",
			wantContains:  []string{"sakila.public"},
			wantDirective: wantDirective,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "pub",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "public",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
		{
			handles:           []string{sakila.My, sakila.Pg},
			withFlagActiveSrc: sakila.Pg,
			arg:               "publ",
			wantContains:      []string{"public"},
			wantDirective:     wantDirective,
		},
		{
			handles:           []string{sakila.Pg, sakila.My},
			withFlagActiveSrc: sakila.My,
			arg:               "",
			wantContains:      []string{"mysql", "sys", "information_schema", "sakila"},
			wantDirective:     wantDirective,
		},
		{
			handles:           []string{sakila.My, sakila.Pg},
			withFlagActiveSrc: sakila.MS,
			arg:               "publ",
			// Should error because sakila.MS isn't a loaded source (via "handles").
			wantDirective: cobra.ShellCompDirectiveError,
		},
	}

	for _, cmdName := range []string{"slq", "sql"} {
		cmdName := cmdName
		t.Run(cmdName, func(t *testing.T) {
			t.Parallel()

			for i, tc := range testCases {
				tc := tc
				t.Run(tu.Name(i, tc.handles, tc.withFlagActiveSrc, tc.arg), func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					tr := testrun.New(th.Context, t, nil)
					for _, handle := range tc.handles {
						tr.Add(*th.Source(handle))
					}

					args := []string{cmdName}
					if tc.withFlagActiveSrc != "" {
						args = append(args, "--"+flag.ActiveSrc, tc.withFlagActiveSrc)
					}
					args = append(args, "--"+flag.ActiveSchema, tc.arg)

					got := testComplete(t, tr, args...)
					assert.Equal(t, tc.wantDirective, got.result,
						"wanted: %v\ngot   : %v",
						cobraz.MarshalDirective(tc.wantDirective),
						cobraz.MarshalDirective(got.result))

					if tc.wantDirective == cobra.ShellCompDirectiveError {
						require.Empty(t, got.values)
					} else {
						for j := range tc.wantContains {
							assert.Contains(t, got.values, tc.wantContains[j])
						}
					}
				})
			}
		})
	}
}

// TestCompleteFlagActiveSchema_inspect tests flag.ActiveSchema
// behavior for the inspect command.
//
// See also: TestCompleteFlagActiveSchema_query_cmds.
func TestCompleteFlagActiveSchema_inspect(t *testing.T) {
	t.Parallel()
	const wantDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

	testCases := []struct {
		handles          []string
		arg              string
		withArgActiveSrc string
		wantContains     []string
		wantDirective    cobra.ShellCompDirective
	}{
		{
			handles:       []string{sakila.Pg},
			arg:           "saki",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "",
			wantContains:  []string{"public", "sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "sakila",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "sakila.pub",
			wantContains:  []string{"sakila.public"},
			wantDirective: wantDirective,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "pub",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
		{
			handles:       []string{sakila.Pg},
			arg:           "public",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
		{
			handles:          []string{sakila.My, sakila.Pg},
			withArgActiveSrc: sakila.Pg,
			arg:              "publ",
			wantContains:     []string{"public"},
			wantDirective:    wantDirective,
		},
		{
			handles:          []string{sakila.Pg, sakila.My},
			withArgActiveSrc: sakila.My,
			arg:              "",
			wantContains:     []string{"mysql", "sys", "information_schema", "sakila"},
			wantDirective:    wantDirective,
		},
		{
			handles:          []string{sakila.My, sakila.Pg},
			withArgActiveSrc: sakila.MS,
			arg:              "publ",
			// Should error because sakila.MS isn't a loaded source (via "handles").
			wantDirective: cobra.ShellCompDirectiveError,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.handles, tc.withArgActiveSrc, tc.arg), func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			for _, handle := range tc.handles {
				tr.Add(*th.Source(handle))
			}

			args := []string{"inspect"}
			if tc.withArgActiveSrc != "" {
				args = append(args, tc.withArgActiveSrc)
			}
			args = append(args, "--"+flag.ActiveSchema, tc.arg)

			got := testComplete(t, tr, args...)
			assert.Equal(t, tc.wantDirective, got.result,
				"wanted: %v\ngot   : %v",
				cobraz.MarshalDirective(tc.wantDirective),
				cobraz.MarshalDirective(got.result))

			if tc.wantDirective == cobra.ShellCompDirectiveError {
				require.Empty(t, got.values)
			} else {
				for j := range tc.wantContains {
					assert.Contains(t, got.values, tc.wantContains[j])
				}
			}
		})
	}
}

func TestCompleteFilterActiveGroup(t *testing.T) {
	const (
		prodInv   = "@prod/inventory"
		prodSales = "@prod/sales"
		devInv    = "@dev/inventory"
		devSales  = "@dev/sales"
	)

	allSrcs := []string{prodInv, prodSales, devInv, devSales}
	prodSrcs := []string{prodInv, prodSales}
	devSrcs := []string{devInv, devSales}
	_ = devSrcs

	testCases := []struct {
		name          string
		srcs          []string
		activeSrc     string
		activeGroup   string
		args          []string
		wantEquals    []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name:          "prod_src_empty",
			srcs:          allSrcs,
			activeSrc:     "",
			activeGroup:   "prod",
			args:          []string{"src", "@"},
			wantEquals:    prodSrcs,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "dev_src_@",
			srcs:          allSrcs,
			activeSrc:     "",
			activeGroup:   "dev",
			args:          []string{"src", "@"},
			wantEquals:    devSrcs,
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil).Hush()

			for _, handle := range tc.srcs {
				src := testh.NewActorSource(t, handle, true)
				tr.Add(*src)
			}

			_, err := tr.Run.Config.Collection.SetActive(tc.activeSrc, false)
			require.NoError(t, err)

			require.NoError(t, tr.Run.Config.Collection.SetActiveGroup(tc.activeGroup))
			require.NoError(t, tr.Run.ConfigStore.Save(tr.Context, tr.Run.Config))

			got := testComplete(t, tr, tc.args...)
			assert.Equal(t, tc.wantDirective, got.result,
				"wanted: %v\ngot   : %v",
				cobraz.MarshalDirective(tc.wantDirective),
				cobraz.MarshalDirective(got.result))

			if tc.wantDirective == cobra.ShellCompDirectiveError {
				require.Empty(t, got.values)
			} else {
				require.Equal(t, tc.wantEquals, got.values)
			}
		})
	}
}

// TestCompleteAllCobraRequestCmds verifies that completion
// works with both cobra.ShellCompRequestCmd and
// cobra.ShellCompNoDescRequestCmd.
func TestCompleteAllCobraRequestCmds(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		args          []string
		wantContains  []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			name:          "slq_empty",
			args:          []string{"@"},
			wantContains:  []string{source.ActiveHandle, sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:          "slq_@",
			args:          []string{"@"},
			wantContains:  []string{source.ActiveHandle, sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:          "src_empty",
			args:          []string{"src", ""},
			wantContains:  []string{sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "src_@",
			args:          []string{"src", "@"},
			wantContains:  []string{sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:          "inspect_empty",
			args:          []string{"inspect", ""},
			wantContains:  []string{sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:          "inspect_@",
			args:          []string{"inspect", "@"},
			wantContains:  []string{sakila.SL3},
			wantDirective: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
	}

	for _, cobraCmd := range []string{cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd} {
		cobraCmd := cobraCmd
		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				t.Run(cobraCmd, func(t *testing.T) {
					t.Parallel()

					th := testh.New(t)
					th.Context = options.NewContext(th.Context, options.Options{cli.OptShellCompletionLog.Key(): true})
					tr := testrun.New(th.Context, t, nil)
					tr.Add(*th.Source(sakila.SL3))

					args := append([]string{cobraCmd}, tc.args...)
					err := tr.Exec(args...)
					require.NoError(t, err)

					got := parseCompletion(tr)
					assert.Equal(t, cobraz.MarshalDirective(tc.wantDirective), cobraz.MarshalDirective(got.result))
					for j := range tc.wantContains {
						assert.Contains(t, got.values, tc.wantContains[j])
					}
				})
			})
		}
	}
}

func enableCompletionLog(ctx context.Context) context.Context {
	o := options.FromContext(ctx)
	if o == nil {
		return options.NewContext(ctx, options.Options{cli.OptShellCompletionLog.Key(): true})
	}

	o[cli.OptShellCompletionLog.Key()] = true
	return ctx
}
