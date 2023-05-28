package cli_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/spf13/cobra"
)

var locSchemes = []string{
	"mysql://",
	"postgres://",
	"sqlserver://",
}

func TestCompleteAddLocation(t *testing.T) {
	testCases := []struct {
		// args will have "add" prepended
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		//{
		//	args: []string{""},
		//	want: locSchemes,
		//},
		//{
		//	args: []string{"p"},
		//	want: []string{"postgres://"},
		//},
		//{
		//	args: []string{"postgres:/"},
		//	want: []string{"postgres://"},
		//},
		//{
		//	args: []string{"postgres://"},
		//	want: []string{"postgres://username"},
		//},
		{
			args: []string{"postgres://alice"},
			want: []string{"postgres://alice:"},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result)
			require.Equal(t, tc.want, got.values)
		})
	}
}

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

	directiveLine := strings.TrimPrefix(strings.TrimSpace(c.stderr), "Completion ended with directive: ")
	directives := strings.Split(directiveLine, ", ")

	for _, d := range directives {
		switch d {
		case "ShellCompDirectiveError":
			c.directives = append(c.directives, cobra.ShellCompDirectiveError)
		case "ShellCompDirectiveNoSpace":
			c.directives = append(c.directives, cobra.ShellCompDirectiveNoSpace)
		case "ShellCompDirectiveNoFileComp":
			c.directives = append(c.directives, cobra.ShellCompDirectiveNoFileComp)
		case "ShellCompDirectiveFilterFileExt":
			c.directives = append(c.directives, cobra.ShellCompDirectiveFilterFileExt)
		case "ShellCompDirectiveFilterDirs":
			c.directives = append(c.directives, cobra.ShellCompDirectiveFilterDirs)
		case "ShellCompDirectiveKeepOrder":
			c.directives = append(c.directives, cobra.ShellCompDirectiveKeepOrder)
		case "ShellCompDirectiveDefault":
			c.directives = append(c.directives, cobra.ShellCompDirectiveDefault)
		default:
			tr.T.Fatalf("Unknown cobra.ShellCompDirective: %s", directiveLine)
		}
	}

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
