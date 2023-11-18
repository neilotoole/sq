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

func TestCompleteFlagActiveSchema(t *testing.T) {
	const wantDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

	testCases := []struct {
		handle        string
		arg           string
		wantContains  []string
		wantDirective cobra.ShellCompDirective
	}{
		{
			handle:        sakila.Pg,
			arg:           "saki",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handle:        sakila.Pg,
			arg:           "",
			wantContains:  []string{"public", "sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handle:        sakila.Pg,
			arg:           "sakila",
			wantContains:  []string{"sakila."},
			wantDirective: wantDirective | cobra.ShellCompDirectiveNoSpace,
		},
		{
			handle:        sakila.Pg,
			arg:           "sakila.pub",
			wantContains:  []string{"sakila.public"},
			wantDirective: wantDirective,
		},
		{
			handle:        sakila.Pg,
			arg:           "pub",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
		{
			handle:        sakila.Pg,
			arg:           "public",
			wantContains:  []string{"public"},
			wantDirective: wantDirective,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.handle, tc.arg), func(t *testing.T) {
			th := testh.New(t)
			src := th.Source(sakila.Pg)
			tr := testrun.New(th.Context, t, nil).Add(*src)

			got := testComplete(t, tr, "slq", "--"+flag.ActiveSchema, tc.arg)
			for i := range tc.wantContains {
				assert.Contains(t, got.values, tc.wantContains[i])
			}

			assert.Equal(t, tc.wantDirective, got.result,
				"wanted: %v\ngot   : %v",
				cobraz.MarshalDirective(tc.wantDirective),
				cobraz.MarshalDirective(got.result))
		})
	}
}
