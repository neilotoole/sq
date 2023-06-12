package cli_test

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"github.com/neilotoole/sq/drivers/sqlserver"

	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/samber/lo"

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
	"sqlite3://",
	"sqlserver://",
}

const stdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

func TestCompleteAddLocation_Postgres(t *testing.T) {
	tutil.SkipWindows(t, "Shell completion not fully implemented for windows")

	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		// args will have "add" prepended
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args: []string{""},
			want: lo.Union(locSchemes, []string{
				"data/", "my/", "my.db", "post/", "post.db", "sqlite/", "sqlite.db",
			}),
			wantResult: stdDirective,
		},
		{
			args:       []string{"p"},
			want:       []string{"postgres://", "post/", "post.db"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"postgres:/"},
			want:       []string{"postgres://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://"},
			want: []string{
				"postgres://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice"},
			want: []string{
				"postgres://alice@",
				"postgres://alice:",
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

func TestCompleteAddLocation_SQLServer(t *testing.T) {
	tutil.SkipWindows(t, "Shell completion not fully implemented for windows")

	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"sqlse"},
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
				"sqlserver://username",
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
			args:       []string{"sqlserver://alice@server/"},
			want:       []string{"sqlserver://alice@server/instance?database="},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlserver://alice@server/instance"},
			want:       []string{"sqlserver://alice@server/instance?database="},
			wantResult: stdDirective,
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

func TestCompleteAddLocation_MySQL(t *testing.T) {
	tutil.SkipWindows(t, "Shell completion not fully implemented for windows")

	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"m"},
			want:       []string{"mysql://", "my/", "my.db"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"my"},
			want:       []string{"mysql://", "my/", "my.db"},
			wantResult: stdDirective,
		},
		{
			// When the input is definitively not a db url, the completion
			// switches to the default shell (file) completion.
			args:       []string{"my/"},
			want:       []string{},
			wantResult: cobra.ShellCompDirectiveDefault,
		},
		{
			args:       []string{"mysql"},
			want:       []string{"mysql://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"mysql:/"},
			want:       []string{"mysql://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://"},
			want: []string{
				"mysql://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice"},
			want: []string{
				"mysql://alice@",
				"mysql://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice:"},
			want: []string{
				"mysql://alice:",
				"mysql://alice:@",
				"mysql://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@"},
			want: []string{
				"mysql://alice@localhost/",
				"mysql://alice@localhost:3306/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@server"},
			want: []string{
				"mysql://alice@server/",
				"mysql://alice@server:3306/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localho"},
			want: []string{
				"mysql://alice@localho/",
				"mysql://alice@localho:3306/",
				"mysql://alice@localhost/",
				"mysql://alice@localhost:3306/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost"},
			want: []string{
				"mysql://alice@localhost/",
				"mysql://alice@localhost:3306/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost:"},
			want: []string{
				"mysql://alice@localhost:3306/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost:80"},
			want: []string{
				"mysql://alice@localhost:80/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/"},
			want: []string{
				"mysql://alice@localhost/db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila"},
			want: []string{
				"mysql://alice@localhost/sakila?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?"},
			want: []string{
				"mysql://alice@localhost/sakila?allowAllFiles=",
				"mysql://alice@localhost/sakila?allowCleartextPasswords=",
				"mysql://alice@localhost/sakila?allowFallbackToPlaintext=",
				"mysql://alice@localhost/sakila?allowNativePasswords=",
				"mysql://alice@localhost/sakila?allowOldPasswords=",
				"mysql://alice@localhost/sakila?charset=",
				"mysql://alice@localhost/sakila?checkConnLiveness=",
				"mysql://alice@localhost/sakila?clientFoundRows=",
				"mysql://alice@localhost/sakila?collation=",
				"mysql://alice@localhost/sakila?columnsWithAlias=",
				"mysql://alice@localhost/sakila?connectionAttributes=",
				"mysql://alice@localhost/sakila?interpolateParams=",
				"mysql://alice@localhost/sakila?loc=",
				"mysql://alice@localhost/sakila?maxAllowedPackage=",
				"mysql://alice@localhost/sakila?multiStatements=",
				"mysql://alice@localhost/sakila?parseTime=",
				"mysql://alice@localhost/sakila?readTimeout=",
				"mysql://alice@localhost/sakila?rejectReadOnly=",
				"mysql://alice@localhost/sakila?timeout=",
				"mysql://alice@localhost/sakila?tls=",
				"mysql://alice@localhost/sakila?writeTimeout=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tl"},
			want: []string{
				"mysql://alice@localhost/sakila?tls=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?a=1&b=2&tl"},
			want: []string{
				"mysql://alice@localhost/sakila?a=1&b=2&tls=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?a=1&b=2&tls"},
			want: []string{
				"mysql://alice@localhost/sakila?a=1&b=2&tls=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls="},
			want: []string{
				"mysql://alice@localhost/sakila?tls=false",
				"mysql://alice@localhost/sakila?tls=true",
				"mysql://alice@localhost/sakila?tls=skip-verify",
				"mysql://alice@localhost/sakila?tls=preferred",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls=s"},
			want: []string{
				"mysql://alice@localhost/sakila?tls=skip-verify",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls=skip-verify"},
			want: []string{
				"mysql://alice@localhost/sakila?tls=skip-verify&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls=skip-verify&lo"},
			want: []string{
				"mysql://alice@localhost/sakila?tls=skip-verify&loc=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls=skip-verify&loc="},
			want: []string{
				"mysql://alice@localhost/sakila?tls=skip-verify&loc=UTC",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost/sakila?tls=skip-verify&loc=UTC"},
			want: []string{
				"mysql://alice@localhost/sakila?tls=skip-verify&loc=UTC&",
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

func TestCompleteAddLocation_SQLite3(t *testing.T) {
	tutil.SkipWindows(t, "Shell completion not fully implemented for windows")

	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"s"},
			want:       []string{"sqlite3://", "sqlserver://", "sqlite/", "sqlite.db"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite"},
			want:       []string{"sqlite3://", "sqlite/", "sqlite.db"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite/"},
			want:       []string{},
			wantResult: cobra.ShellCompDirectiveDefault,
		},
		{
			args:       []string{"my/my_"},
			want:       []string{},
			wantResult: cobra.ShellCompDirectiveDefault,
		},
		{
			args:       []string{"sqlite3:"},
			want:       []string{"sqlite3://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3:/"},
			want:       []string{"sqlite3://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://"},
			want: []string{
				"sqlite3://data/",
				"sqlite3://my/",
				"sqlite3://my.db",
				"sqlite3://post/",
				"sqlite3://post.db",
				"sqlite3://sqlite/",
				"sqlite3://sqlite.db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://my"},
			want: []string{
				"sqlite3://my/",
				"sqlite3://my.db",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.d"},
			want:       []string{"sqlite3://my.db"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db"},
			want:       []string{"sqlite3://my.db?"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://data/nest1/data.db"},
			want:       []string{"sqlite3://data/nest1/data.db?", "sqlite3://data/nest1/data.db2"},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://my.db?"},
			want: []string{
				"sqlite3://my.db?_auth=",
				"sqlite3://my.db?_auth_crypt=",
				"sqlite3://my.db?_auth_pass=",
				"sqlite3://my.db?_auth_salt=",
				"sqlite3://my.db?_auth_user=",
				"sqlite3://my.db?_auto_vacuum=",
				"sqlite3://my.db?_busy_timeout=",
				"sqlite3://my.db?_cache_size=",
				"sqlite3://my.db?_case_sensitive_like=",
				"sqlite3://my.db?_defer_foreign_keys=",
				"sqlite3://my.db?_foreign_keys=",
				"sqlite3://my.db?_ignore_check_constraints=",
				"sqlite3://my.db?_journal_mode=",
				"sqlite3://my.db?_loc=",
				"sqlite3://my.db?_locking_mode=",
				"sqlite3://my.db?_mutex=",
				"sqlite3://my.db?_query_only=",
				"sqlite3://my.db?_recursive_triggers=",
				"sqlite3://my.db?_secure_delete=",
				"sqlite3://my.db?_synchronous=",
				"sqlite3://my.db?_txlock=",
				"sqlite3://my.db?cache=",
				"sqlite3://my.db?mode=",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db?_locking_"},
			want:       []string{"sqlite3://my.db?_locking_mode="},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://my.db?_locking_mode="},
			want: []string{
				"sqlite3://my.db?_locking_mode=NORMAL",
				"sqlite3://my.db?_locking_mode=EXCLUSIVE",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db?_locking_mode=NORM"},
			want:       []string{"sqlite3://my.db?_locking_mode=NORMAL"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db?_locking_mode=NORMAL"},
			want:       []string{"sqlite3://my.db?_locking_mode=NORMAL&"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db?_locking_mode=NORMAL"},
			want:       []string{"sqlite3://my.db?_locking_mode=NORMAL&"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3://my.db?_locking_mode=NORMAL&ca"},
			want:       []string{"sqlite3://my.db?_locking_mode=NORMAL&cache="},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://my.db?_locking_mode=NORMAL&cache="},
			want: []string{
				"sqlite3://my.db?_locking_mode=NORMAL&cache=true",
				"sqlite3://my.db?_locking_mode=NORMAL&cache=false",
				"sqlite3://my.db?_locking_mode=NORMAL&cache=FAST",
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

func TestCompleteAddLocation_History_Postgres(t *testing.T) {
	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	tr.Add(
		source.Source{
			Handle:   "@src1",
			Type:     postgres.Type,
			Location: "postgres://alice:abc123@dev.acme.com:7777/sakila?application_name=app1&channel_binding=prefer",
		},
		source.Source{
			Handle:   "@src2",
			Type:     postgres.Type,
			Location: "postgres://bob:abc123@prod.acme.com:8888/sales?application_name=app2&channel_binding=require",
		},
	)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args: []string{"postgres://"},
			want: []string{
				"postgres://alice",
				"postgres://bob",
				"postgres://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://a"},
			want: []string{
				"postgres://a@",
				"postgres://a:",
				"postgres://alice@",
				"postgres://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice"},
			want: []string{
				"postgres://alice@",
				"postgres://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost:5432/",
				"postgres://alice@dev.acme.com:7777/",
				"postgres://alice@prod.acme.com:8888/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev"},
			want: []string{
				"postgres://alice@dev/",
				"postgres://alice@dev:5432/",
				"postgres://alice@dev.acme.com:7777/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com"},
			want: []string{
				"postgres://alice@dev.acme.com/",
				"postgres://alice@dev.acme.com:5432/",
				"postgres://alice@dev.acme.com:7777/",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com/"},
			want: []string{
				"postgres://alice@dev.acme.com/db",
				"postgres://alice@dev.acme.com/sakila",
				"postgres://alice@dev.acme.com/sales",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com/sa"},
			want: []string{
				"postgres://alice@dev.acme.com/sa?",
				"postgres://alice@dev.acme.com/sakila",
				"postgres://alice@dev.acme.com/sales",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com/sakila?"},
			want: []string{
				"postgres://alice@dev.acme.com/sakila?application_name=app1&channel_binding=prefer",
				"postgres://alice@dev.acme.com/sakila?application_name=",
				"postgres://alice@dev.acme.com/sakila?channel_binding=",
				"postgres://alice@dev.acme.com/sakila?connect_timeout=",
				"postgres://alice@dev.acme.com/sakila?fallback_application_name=",
				"postgres://alice@dev.acme.com/sakila?gssencmode=",
				"postgres://alice@dev.acme.com/sakila?sslmode=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com/sakila?app"},
			want: []string{
				"postgres://alice@dev.acme.com/sakila?application_name=app1&channel_binding=prefer",
				"postgres://alice@dev.acme.com/sakila?application_name=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			assert.Equal(t, tc.wantResult, got.result, got.directives)
			assert.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_History_SQLServer(t *testing.T) {
	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	tr.Add(
		source.Source{
			Handle:   "@src1",
			Type:     sqlserver.Type,
			Location: "sqlserver://alice:abc123@dev.acme.com:7777?database=sakila&app+name=app1&encrypt=disable",
		},
		source.Source{
			Handle:   "@src2",
			Type:     sqlserver.Type,
			Location: "sqlserver://bob:abc123@prod.acme.com:8888?database=sales&app+name=app2&encrypt=true",
		},
	)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args: []string{"sqlserver://"},
			want: []string{
				"sqlserver://alice",
				"sqlserver://bob",
				"sqlserver://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://a"},
			want: []string{
				"sqlserver://a@",
				"sqlserver://a:",
				"sqlserver://alice@",
				"sqlserver://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice"},
			want: []string{
				"sqlserver://alice@",
				"sqlserver://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@"},
			want: []string{
				"sqlserver://alice@localhost?database=",
				"sqlserver://alice@localhost:1433?database=",
				"sqlserver://alice@dev.acme.com:7777?database=",
				"sqlserver://alice@prod.acme.com:8888?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev"},
			want: []string{
				"sqlserver://alice@dev?database=",
				"sqlserver://alice@dev:1433?database=",
				"sqlserver://alice@dev.acme.com:7777?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=",
				"sqlserver://alice@dev.acme.com:1433?database=",
				"sqlserver://alice@dev.acme.com:7777?database=",
			},
			wantResult: stdDirective,
		},
		// FIXME: Deal with /instance/
		{
			args: []string{"sqlserver://alice@dev.acme.com?"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@dev.acme.com?database=",
				"sqlserver://alice@dev.acme.com?ApplicationIntent=",
				"sqlserver://alice@dev.acme.com?ServerSPN=",
				"sqlserver://alice@dev.acme.com?TrustServerCertificate=",
				"sqlserver://alice@dev.acme.com?Workstation+ID=",
				"sqlserver://alice@dev.acme.com?app+name=",
				"sqlserver://alice@dev.acme.com?certificate=",
				"sqlserver://alice@dev.acme.com?connection+timeout=",
				"sqlserver://alice@dev.acme.com?dial+timeout=",
				"sqlserver://alice@dev.acme.com?encrypt=",
				"sqlserver://alice@dev.acme.com?failoverpartner=",
				"sqlserver://alice@dev.acme.com?failoverport=",
				"sqlserver://alice@dev.acme.com?hostNameInCertificate=",
				"sqlserver://alice@dev.acme.com?keepAlive=",
				"sqlserver://alice@dev.acme.com?log=",
				"sqlserver://alice@dev.acme.com?packet+size=",
				"sqlserver://alice@dev.acme.com?protocol=",
				"sqlserver://alice@dev.acme.com?tlsmin=",
				"sqlserver://alice@dev.acme.com?user+id=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?data"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@dev.acme.com?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@dev.acme.com?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database="},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sales&app+name=app2&encrypt=true",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sa"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@dev.acme.com?database=sa&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=saki"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=saki&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sakila"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sakila&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sakila&app"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			assert.Equal(t, tc.wantResult, got.result, got.directives)
			assert.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_History_SQLLite3(t *testing.T) {
	wd := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)
	src3Loc := "sqlite3://" + wd + "/my.db?cache=FAST"

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle: "@src2",
			Type:   sqlite3.Type,
			// Note that this file doesn't actually exist
			Location: "sqlite3:///__dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
		},
		source.Source{
			Handle: "@src1",
			Type:   sqlite3.Type,
			// Note that this file doesn't actually exist
			Location: "sqlite3:///__dir1/sqtest/sq/src1.db",
		},
		source.Source{
			Handle: "@src3",
			Type:   sqlite3.Type,
			// This file DOES exist
			Location: src3Loc,
		},
	)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args: []string{"sqlite3://"},
			want: []string{
				src3Loc,
				"sqlite3:///__dir1/sqtest/sq/src1.db",
				"sqlite3:///__dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
				"sqlite3://data/",
				"sqlite3://my/",
				"sqlite3://my.db",
				"sqlite3://post/",
				"sqlite3://post.db",
				"sqlite3://sqlite/",
				"sqlite3://sqlite.db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3://my"},
			want: []string{
				"sqlite3://my/",
				"sqlite3://my.db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3:///__dir1/sqtest/"},
			want: []string{
				"sqlite3:///__dir1/sqtest/sq/src1.db",
				"sqlite3:///__dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3:///__dir1/sqtest/sq/not_a_dir"},
			want:       []string{},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3:///__dir1/sqtest/sq/src"},
			want: []string{
				"sqlite3:///__dir1/sqtest/sq/src1.db",
				"sqlite3:///__dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3:///__dir1/sqtest/sq/src1.db"},
			want:       []string{}, // Empty because file doesn't actually exist
			wantResult: stdDirective,
		},
		{
			args:       []string{src3Loc},
			want:       []string{src3Loc + "&"},
			wantResult: stdDirective,
		},
		{
			args: []string{src3Loc + "&"},
			want: []string{
				src3Loc + "&_auth=",
				src3Loc + "&_auth_crypt=",
				src3Loc + "&_auth_pass=",
				src3Loc + "&_auth_salt=",
				src3Loc + "&_auth_user=",
				src3Loc + "&_auto_vacuum=",
				src3Loc + "&_busy_timeout=",
				src3Loc + "&_cache_size=",
				src3Loc + "&_case_sensitive_like=",
				src3Loc + "&_defer_foreign_keys=",
				src3Loc + "&_foreign_keys=",
				src3Loc + "&_ignore_check_constraints=",
				src3Loc + "&_journal_mode=",
				src3Loc + "&_loc=",
				src3Loc + "&_locking_mode=",
				src3Loc + "&_mutex=",
				src3Loc + "&_query_only=",
				src3Loc + "&_recursive_triggers=",
				src3Loc + "&_secure_delete=",
				src3Loc + "&_synchronous=",
				src3Loc + "&_txlock=",
				src3Loc + "&mode=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			assert.Equal(t, tc.wantResult, got.result, got.directives)
			assert.Equal(t, tc.want, got.values)
		})
	}
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

func TestDoCompleteAddLocationFile(t *testing.T) {
	tutil.SkipWindows(t, "Shell completion not fully implemented for windows")

	absDir := tutil.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", absDir)

	testCases := []struct {
		in   string
		want []string
	}{
		{"", []string{"data/", "my/", "my.db", "post/", "post.db", "sqlite/", "sqlite.db"}},
		{"m", []string{"my/", "my.db"}},
		{"my", []string{"my/", "my.db"}},
		{"my/", []string{"my/my1.db", "my/my_nest/"}},
		{"my/my", []string{"my/my1.db", "my/my_nest/"}},
		{"my/my1", []string{"my/my1.db"}},
		{"my/my1.db", []string{"my/my1.db"}},
		{"my/my_nes", []string{"my/my_nest/"}},
		{"my/my_nest", []string{"my/my_nest/"}},
		{"my/my_nest/", []string{"my/my_nest/my2.db"}},
		{"data/nest1/", []string{"data/nest1/data.db", "data/nest1/data.db2"}},
		{"data/nest1/data.db", []string{"data/nest1/data.db", "data/nest1/data.db2"}},
		{
			absDir + "/",
			stringz.PrefixSlice([]string{
				"data/", "my/", "my.db", "post/", "post.db", "sqlite/", "sqlite.db",
			}, absDir+"/"),
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.in), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), slogt.New(t))
			t.Logf("input: %s", tc.in)
			t.Logf("want:  %s", tc.want)
			got := cli.DoCompleteAddLocationFile(ctx, tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
