package cli_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/testh"

	"github.com/neilotoole/sq/cli"

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
const stdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

func TestCompleteAddLocation_postgres(t *testing.T) {
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
			args: []string{"postgres://alice@localhost/sakila"},
			want: []string{
				"postgres://alice@localhost/sakila?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?"},
			want: []string{
				"postgres://alice@localhost/sakila?application_name=",
				"postgres://alice@localhost/sakila?channel_binding=",
				"postgres://alice@localhost/sakila?connect_timeout=",
				"postgres://alice@localhost/sakila?fallback_application_name=",
				"postgres://alice@localhost/sakila?gssencmode=",
				"postgres://alice@localhost/sakila?sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?ss"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?a=1&b=2&ss"},
			want: []string{
				"postgres://alice@localhost/sakila?a=1&b=2&sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?a=1&b=2&sslmode"},
			want: []string{
				"postgres://alice@localhost/sakila?a=1&b=2&sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode="},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=disable",
				"postgres://alice@localhost/sakila?sslmode=allow",
				"postgres://alice@localhost/sakila?sslmode=prefer",
				"postgres://alice@localhost/sakila?sslmode=require",
				"postgres://alice@localhost/sakila?sslmode=verify-ca",
				"postgres://alice@localhost/sakila?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=v"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=verify-ca",
				"postgres://alice@localhost/sakila?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=verify-"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=verify-ca",
				"postgres://alice@localhost/sakila?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=verify-ful"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=verify-full",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=verify-full"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=verify-full&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=verify-full-something"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=verify-full-something&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost/sakila?sslmode=disable"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=disable&",
			},
			wantResult: stdDirective,
		},
		{
			// Note the extra "?", which apparently is valid
			args:       []string{"postgres://alice@localhost/sakila?sslmode=disable?"},
			want:       []string{"postgres://alice@localhost/sakila?sslmode=disable?&"},
			wantResult: stdDirective,
		},
		{
			// Being that sslmode is already specified, it should not appear a
			// second time.
			args: []string{"postgres://alice@localhost/sakila?sslmode=disable&"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=disable&application_name=",
				"postgres://alice@localhost/sakila?sslmode=disable&channel_binding=",
				"postgres://alice@localhost/sakila?sslmode=disable&connect_timeout=",
				"postgres://alice@localhost/sakila?sslmode=disable&fallback_application_name=",
				"postgres://alice@localhost/sakila?sslmode=disable&gssencmode=",
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

func TestCompleteAddLocation_sqlserver(t *testing.T) {
	//th := testh.New(t)
	//drvr, err := th.Registry().DriverFor(sqlserver.Type)
	//require.NoError(t, err)
	//sqlDrvr, ok := drvr.(driver.SQLDriver)
	//require.True(t, ok)
	//

	testCases := []struct {
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
			args:       []string{"s"},
			want:       []string{"sqlserver://"},
			wantResult: stdDirective,
		},

		{
			args:       []string{"sqlserver:/"},
			want:       []string{"sqlserver://"},
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
			args: []string{"sqlserver://alice@server"},
			want: []string{
				"sqlserver://alice@server?database=",
				"sqlserver://alice@server:1433?database=",
			},
			wantResult: stdDirective,
		},
		{
			// This is an error, because it should be @server?dbname=
			// FIXME: Actually, this should be allowed.
			// sqlserver://username:password@host/instance?param1=value&param2=value
			args:       []string{"sqlserver://alice@server/"},
			want:       []string{},
			wantResult: cobra.ShellCompDirectiveError,
		},
		{
			args: []string{"sqlserver://alice@server?"},
			want: []string{
				"sqlserver://alice@server?database=",
				"sqlserver://alice@server?ApplicationIntent=",
				"sqlserver://alice@server?ServerSPN=",
				"sqlserver://alice@server?TrustServerCertificate=",
				"sqlserver://alice@server?Workstation+ID=",
				"sqlserver://alice@server?app+name=",
				"sqlserver://alice@server?certificate=",
				"sqlserver://alice@server?connection+timeout=",
				"sqlserver://alice@server?dial+timeout=",
				"sqlserver://alice@server?encrypt=",
				"sqlserver://alice@server?failoverpartner=",
				"sqlserver://alice@server?failoverport=",
				"sqlserver://alice@server?hostNameInCertificate=",
				"sqlserver://alice@server?keepAlive=",
				"sqlserver://alice@server?log=",
				"sqlserver://alice@server?packet+size=",
				"sqlserver://alice@server?protocol=",
				"sqlserver://alice@server?tlsmin=",
				"sqlserver://alice@server?user+id=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?da"},
			want: []string{
				"sqlserver://alice@server?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?database"},
			want: []string{
				"sqlserver://alice@server?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?database=sakila"},
			want: []string{
				"sqlserver://alice@server?database=sakila&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?database=sakila&tls"},
			want: []string{
				"sqlserver://alice@server?database=sakila&tlsmin=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?database=sakila&tlsmin"},
			want: []string{
				"sqlserver://alice@server?database=sakila&tlsmin=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server?database=sakila&tlsmin="},
			want: []string{
				"sqlserver://alice@server?database=sakila&tlsmin=1.0",
				"sqlserver://alice@server?database=sakila&tlsmin=1.1",
				"sqlserver://alice@server?database=sakila&tlsmin=1.2",
				"sqlserver://alice@server?database=sakila&tlsmin=1.3",
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

func TestParseLoc_stage(t *testing.T) {
	testCases := []struct {
		loc  string
		want cli.PlocStage
	}{
		{"", cli.PlocInit},
		{"postgres", cli.PlocInit},
		{"postgres:/", cli.PlocInit},
		{"postgres://", cli.PlocScheme},
		{"postgres://alice", cli.PlocScheme},
		{"postgres://alice:", cli.PlocUser},
		{"postgres://alice:pass", cli.PlocUser},
		{"postgres://alice:pass@", cli.PlocPass},
		{"postgres://alice:@", cli.PlocPass},
		{"postgres://alice@", cli.PlocPass},
		{"postgres://alice@localhost", cli.PlocPass},
		{"postgres://alice:@localhost", cli.PlocPass},
		{"postgres://alice:pass@localhost", cli.PlocPass},
		{"postgres://alice@localhost:", cli.PlocHostname},
		{"postgres://alice:@localhost:", cli.PlocHostname},
		{"postgres://alice:pass@localhost:", cli.PlocHostname},
		{"postgres://alice@localhost:5432", cli.PlocHostname},
		{"postgres://alice@localhost:5432/", cli.PlocHost},
		{"postgres://alice@localhost:5432/s", cli.PlocHost},
		{"postgres://alice@localhost:5432/sakila", cli.PlocHost},
		{"postgres://alice@localhost:5432/sakila?", cli.PlocPath},
		{"postgres://alice@localhost:5432/sakila?sslmode=verify-ca", cli.PlocPath},
		{"postgres://alice:@localhost:5432/sakila?sslmode=verify-ca", cli.PlocPath},
		{"postgres://alice:pass@localhost:5432/sakila?sslmode=verify-ca", cli.PlocPath},
		{"sqlserver://alice:pass@localhost?", cli.PlocPath},
	}

	/*
		sq add postgres://sakila:p_ssW0rd@192.168.50.132/sakila
		sq add postgres://sakila:p_ssW0rd@192.168.50.132/sakila?sslmode=verify-ca
		sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila
		sq add sqlserver://sakila:p_ssW0rd@192.168.50.130\?database=sakila&\keepAlive=30

	*/

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.loc), func(t *testing.T) {
			th := testh.New(t)
			ru := th.Run()

			gotStage, err := cli.DoTestParseLocStage(t, ru, tc.loc)
			require.NoError(t, err)
			require.Equal(t, tc.want, gotStage)
		})
	}
}
