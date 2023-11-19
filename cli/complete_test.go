package cli_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/cli/cobraz"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/spf13/cobra"
)

// testComplete is a helper for testing cobra completion.
func testComplete(t testing.TB, from *testrun.TestRun, args ...string) completion {
	ctx := lg.NewContext(context.Background(), slogt.New(t))

	tr := testrun.New(ctx, t, from)
	args = append([]string{"__complete"}, args...)

	err := tr.Exec(args...)
	require.NoError(t, err)

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
func TestCompleteFlagActiveSchema_query_cmds(t *testing.T) {
	t.Parallel()
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
				t.Run(tutil.Name(i, tc.handles, tc.withFlagActiveSrc, tc.arg), func(t *testing.T) {
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
		t.Run(tutil.Name(i, tc.handles, tc.withArgActiveSrc, tc.arg), func(t *testing.T) {
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
