package cli_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/sq/cli/cobraz"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/lg"

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

// th := testh.New(t)
// drvrQueryParams := map[source.DriverType]map[string][]string{}
// for _, typ := range []source.DriverType{
// postgres.Type,
// mysql.Type,
// sqlite3.Type,
// sqlserver.Type,
// } {
// drvr, _ := th.Registry().DriverFor(typ)
// drvrQueryParams[typ] = drvr.(driver.SQLDriver).ConnParams()
// }

func TestCompleteAddLocation(t *testing.T) {
	const stdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

	testCases := []struct {
		// args will have "add" prepended
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{""},
			want:       locSchemes,
			wantResult: stdDirective,
		},
		{
			args:       []string{"p"},
			want:       []string{"postgres://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"s"},
			want:       []string{"sqlserver://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"postgres:/"},
			want:       []string{"postgres://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlserver:/"},
			want:       []string{"sqlserver://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://"},
			want: []string{
				"postgres://",
				"postgres://username",
				"postgres://username:password",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://"},
			want: []string{
				"sqlserver://",
				"sqlserver://username",
				"sqlserver://username:password",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice"},
			want: []string{
				"postgres://alice@",
				"postgres://alice:",
				"postgres://alice:@",
				"postgres://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice:"},
			want: []string{
				"postgres://alice:",
				"postgres://alice:@",
				"postgres://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost:5432/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@server"},
			want: []string{
				"postgres://alice@server/",
				"postgres://alice@server:5432/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server"},
			want: []string{
				"sqlserver://alice@server/",
				"sqlserver://alice@server:1433/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localho"},
			want: []string{
				"postgres://alice@localho/",
				"postgres://alice@localho:5432/",
				"postgres://alice@localhost/",
				"postgres://alice@localhost:5432/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost:5432/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost:"},
			want: []string{
				"postgres://alice@localhost:5432/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost:80"},
			want: []string{
				"postgres://alice@localhost:80/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/"},
			want: []string{
				"postgres://alice@localhost/db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales"},
			want: []string{
				"postgres://alice@localhost/sales?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?"},
			want: []string{
				"postgres://alice@localhost/sales?application_name=",
				"postgres://alice@localhost/sales?channel_binding=",
				"postgres://alice@localhost/sales?connect_timeout=",
				"postgres://alice@localhost/sales?fallback_application_name=",
				"postgres://alice@localhost/sales?gssencmode=",
				"postgres://alice@localhost/sales?sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?ss"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?a=1&b=2&ss"},
			want: []string{
				"postgres://alice@localhost/sales?a=1&b=2&sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?a=1&b=2&sslmode"},
			want: []string{
				"postgres://alice@localhost/sales?a=1&b=2&sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode="},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=disable",
				"postgres://alice@localhost/sales?sslmode=allow",
				"postgres://alice@localhost/sales?sslmode=prefer",
				"postgres://alice@localhost/sales?sslmode=require",
				"postgres://alice@localhost/sales?sslmode=verify-ca",
				"postgres://alice@localhost/sales?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=v"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=verify-ca",
				"postgres://alice@localhost/sales?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=verify-"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=verify-ca",
				"postgres://alice@localhost/sales?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=verify-ful"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=verify-full"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=verify-full&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=verify-full-something"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=verify-full-something&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sales?sslmode=disable"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=disable&",
			},
			wantResult: stdDirective,
		},
		{
			// Note the extra "?", which apparently is valid
			args:       []string{"postgres://alice@localhost/sales?sslmode=disable?"},
			want:       []string{"postgres://alice@localhost/sales?sslmode=disable?&"},
			wantResult: stdDirective,
		},
		{
			// Being that sslmode is already specified, it should not appear a
			// second time.
			args: []string{"postgres://alice@localhost/sales?sslmode=disable&"},
			want: []string{
				"postgres://alice@localhost/sales?sslmode=disable&application_name=",
				"postgres://alice@localhost/sales?sslmode=disable&channel_binding=",
				"postgres://alice@localhost/sales?sslmode=disable&connect_timeout=",
				"postgres://alice@localhost/sales?sslmode=disable&fallback_application_name=",
				"postgres://alice@localhost/sales?sslmode=disable&gssencmode=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			assert.Equal(t, tc.wantResult, got.result, got.directives)
			assert.Equal(t, tc.want, got.values)
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
