package cli_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/tu"
)

var locSchemes = []string{
	"clickhouse://",
	"duckdb://",
	"mysql://",
	"oracle://",
	"postgres://",
	"rqlite://",
	"sqlite3://",
	"sqlserver://",
}

const stdDirective = cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveKeepOrder

func TestCompleteAddLocation_Postgres(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
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
				"postgres://alice:@",
				"postgres://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost?",
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@server"},
			want: []string{
				"postgres://alice@server/",
				"postgres://alice@server?",
				"postgres://alice@server:5432/",
				"postgres://alice@server:5432?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localho"},
			want: []string{
				"postgres://alice@localho/",
				"postgres://alice@localho?",
				"postgres://alice@localho:5432/",
				"postgres://alice@localho:5432?",
				"postgres://alice@localhost/",
				"postgres://alice@localhost?",
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost?",
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost:"},
			want: []string{
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@localhost:80"},
			want: []string{
				"postgres://alice@localhost:80/",
				"postgres://alice@localhost:80?",
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
			// The second "?" is part of the sslmode value (it's not a
			// query-string delimiter), so the typed value is
			// "disable?", which doesn't match any known sslmode value.
			// The completer offers "&" to push to the next param.
			args: []string{"postgres://alice@localhost/sakila?sslmode=disable?"},
			want: []string{
				"postgres://alice@localhost/sakila?sslmode=disable?&",
			},
			wantResult: stdDirective,
		},
		{
			// gh792: the '@' is part of the query value, not a
			// credentials terminator. The typed value "me@" has no known
			// completions, so the completer offers "&" to move to the
			// next param; it must NOT emit authority-style garbage like
			// "postgres://localhost/db?application_name=me@localhost/".
			args: []string{"postgres://localhost/db?application_name=me@"},
			want: []string{
				"postgres://localhost/db?application_name=me@&",
			},
			wantResult: stdDirective,
		},
		{
			// gh792: an '@' in a path segment is not a credentials
			// terminator either.
			args: []string{"postgres://localhost/cust@"},
			want: []string{
				"postgres://localhost/cust@?",
			},
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
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_SQLServer(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
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
				"sqlserver://alice@server/",
				"sqlserver://alice@server?",
				"sqlserver://alice@server:1433/",
				"sqlserver://alice@server:1433?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@server/"},
			want: []string{
				"sqlserver://alice@server/instance",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlserver://alice@server/instance"},
			want:       []string{"sqlserver://alice@server/instance?"},
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
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

// TestCompleteAddLocation_Rqlite is a smoke test for the rqlite
// completion path: scheme partial to "rqlite://", and
// query-param completion driven by ConnParams (level,
// disableClusterDiscovery, tls, insecure). rqlite is networked
// (no file path), so file-enumeration cases are not exercised here.
func TestCompleteAddLocation_Rqlite(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"rqlite:"},
			want:       []string{"rqlite://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"rqlite:/"},
			want:       []string{"rqlite://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"rqlite://"},
			want: []string{
				"rqlite://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"rqlite://alice@host:4001?"},
			want: []string{
				"rqlite://alice@host:4001?disableClusterDiscovery=",
				"rqlite://alice@host:4001?insecure=",
				"rqlite://alice@host:4001?level=",
				"rqlite://alice@host:4001?timeout=",
				"rqlite://alice@host:4001?tls=",
			},
			wantResult: stdDirective,
		},
		{
			// Values for "level=" are returned in ConnParams declaration
			// order (none, weak, linearizable, strong), not sorted.
			args: []string{"rqlite://alice@host:4001?level="},
			want: []string{
				"rqlite://alice@host:4001?level=none",
				"rqlite://alice@host:4001?level=weak",
				"rqlite://alice@host:4001?level=linearizable",
				"rqlite://alice@host:4001?level=strong",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_MySQL(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
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
				"mysql://alice:@",
				"mysql://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@"},
			want: []string{
				"mysql://alice@localhost/",
				"mysql://alice@localhost?",
				"mysql://alice@localhost:3306/",
				"mysql://alice@localhost:3306?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@server"},
			want: []string{
				"mysql://alice@server/",
				"mysql://alice@server?",
				"mysql://alice@server:3306/",
				"mysql://alice@server:3306?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localho"},
			want: []string{
				"mysql://alice@localho/",
				"mysql://alice@localho?",
				"mysql://alice@localho:3306/",
				"mysql://alice@localho:3306?",
				"mysql://alice@localhost/",
				"mysql://alice@localhost?",
				"mysql://alice@localhost:3306/",
				"mysql://alice@localhost:3306?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost"},
			want: []string{
				"mysql://alice@localhost/",
				"mysql://alice@localhost?",
				"mysql://alice@localhost:3306/",
				"mysql://alice@localhost:3306?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost:"},
			want: []string{
				"mysql://alice@localhost:3306/",
				"mysql://alice@localhost:3306?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"mysql://alice@localhost:80"},
			want: []string{
				"mysql://alice@localhost:80/",
				"mysql://alice@localhost:80?",
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
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_SQLite3(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
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
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

// TestCompleteAddLocation_DuckDB is a smoke test for the DuckDB completion
// path: scheme partial → "duckdb://", file enumeration under the prefix,
// and query-param completion driven by ConnParams. Mirrors the shape of
// TestCompleteAddLocation_SQLite3 but smaller (the SQLite test covers the
// shared file-based driver completion code path exhaustively).
func TestCompleteAddLocation_DuckDB(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location_duck"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"d"},
			want:       []string{"duckdb://", "duck.duckdb"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"du"},
			want:       []string{"duckdb://", "duck.duckdb"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"duckdb:"},
			want:       []string{"duckdb://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"duckdb:/"},
			want:       []string{"duckdb://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"duckdb://"},
			want: []string{
				"duckdb://?",
				"duckdb://duck.duckdb",
				"duckdb://other.duckdb",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"duckdb://duck"},
			want:       []string{"duckdb://duck.duckdb"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"duckdb://duck.duckdb"},
			want:       []string{"duckdb://duck.duckdb?"},
			wantResult: stdDirective,
		},
		{
			args: []string{"duckdb://duck.duckdb?"},
			want: []string{
				"duckdb://duck.duckdb?access_mode=",
				"duckdb://duck.duckdb?default_null_order=",
				"duckdb://duck.duckdb?default_order=",
				"duckdb://duck.duckdb?enable_external_access=",
				"duckdb://duck.duckdb?enable_object_cache=",
				"duckdb://duck.duckdb?memory_limit=",
				"duckdb://duck.duckdb?temp_directory=",
				"duckdb://duck.duckdb?threads=",
				"duckdb://duck.duckdb?wal_autocheckpoint=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"duckdb://duck.duckdb?access_mode="},
			want: []string{
				"duckdb://duck.duckdb?access_mode=READ_ONLY",
				"duckdb://duck.duckdb?access_mode=READ_WRITE",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"duckdb://duck.duckdb?access_mode=READ_ONLY"},
			want:       []string{"duckdb://duck.duckdb?access_mode=READ_ONLY&"},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_ClickHouse(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"c"},
			want:       []string{"clickhouse://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"clickhouse:/"},
			want:       []string{"clickhouse://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://"},
			want: []string{
				"clickhouse://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice"},
			want: []string{
				"clickhouse://alice@",
				"clickhouse://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice:"},
			want: []string{
				"clickhouse://alice:@",
				"clickhouse://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@"},
			want: []string{
				"clickhouse://alice@localhost/",
				"clickhouse://alice@localhost?",
				"clickhouse://alice@localhost:9000/",
				"clickhouse://alice@localhost:9000?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localho"},
			want: []string{
				"clickhouse://alice@localho/",
				"clickhouse://alice@localho?",
				"clickhouse://alice@localho:9000/",
				"clickhouse://alice@localho:9000?",
				"clickhouse://alice@localhost/",
				"clickhouse://alice@localhost?",
				"clickhouse://alice@localhost:9000/",
				"clickhouse://alice@localhost:9000?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost"},
			want: []string{
				"clickhouse://alice@localhost/",
				"clickhouse://alice@localhost?",
				"clickhouse://alice@localhost:9000/",
				"clickhouse://alice@localhost:9000?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost:"},
			want: []string{
				"clickhouse://alice@localhost:9000/",
				"clickhouse://alice@localhost:9000?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost:9000"},
			want: []string{
				"clickhouse://alice@localhost:9000/",
				"clickhouse://alice@localhost:9000?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost:9000/"},
			want: []string{
				"clickhouse://alice@localhost:9000/db",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost:9000/db"},
			want: []string{
				"clickhouse://alice@localhost:9000/db?",
			},
			wantResult: stdDirective,
		},
		{
			// gh743: a bare host with "?" should suggest conn-param keys,
			// not credential placeholders.
			args: []string{"clickhouse://localhost:9000?"},
			want: []string{
				"clickhouse://localhost:9000?compress=",
				"clickhouse://localhost:9000?conn_max_lifetime=",
				"clickhouse://localhost:9000?connection_open_strategy=",
				"clickhouse://localhost:9000?dial_timeout=",
				"clickhouse://localhost:9000?max_idle_conns=",
				"clickhouse://localhost:9000?max_open_conns=",
				"clickhouse://localhost:9000?secure=",
				"clickhouse://localhost:9000?skip_verify=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost/db?"},
			want: []string{
				"clickhouse://alice@localhost/db?compress=",
				"clickhouse://alice@localhost/db?conn_max_lifetime=",
				"clickhouse://alice@localhost/db?connection_open_strategy=",
				"clickhouse://alice@localhost/db?dial_timeout=",
				"clickhouse://alice@localhost/db?max_idle_conns=",
				"clickhouse://alice@localhost/db?max_open_conns=",
				"clickhouse://alice@localhost/db?secure=",
				"clickhouse://alice@localhost/db?skip_verify=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"clickhouse://alice@localhost/db?secure="},
			want: []string{
				"clickhouse://alice@localhost/db?secure=true",
				"clickhouse://alice@localhost/db?secure=false",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_Oracle(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	testCases := []struct {
		args       []string
		want       []string
		wantResult cobra.ShellCompDirective
	}{
		{
			args:       []string{"o"},
			want:       []string{"oracle://"},
			wantResult: stdDirective,
		},
		{
			args:       []string{"oracle:/"},
			want:       []string{"oracle://"},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://"},
			want: []string{
				"oracle://username",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice"},
			want: []string{
				"oracle://alice@",
				"oracle://alice:",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice:"},
			want: []string{
				"oracle://alice:@",
				"oracle://alice:password@",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@"},
			want: []string{
				"oracle://alice@localhost/",
				"oracle://alice@localhost?",
				"oracle://alice@localhost:1521/",
				"oracle://alice@localhost:1521?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localho"},
			want: []string{
				"oracle://alice@localho/",
				"oracle://alice@localho?",
				"oracle://alice@localho:1521/",
				"oracle://alice@localho:1521?",
				"oracle://alice@localhost/",
				"oracle://alice@localhost?",
				"oracle://alice@localhost:1521/",
				"oracle://alice@localhost:1521?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost"},
			want: []string{
				"oracle://alice@localhost/",
				"oracle://alice@localhost?",
				"oracle://alice@localhost:1521/",
				"oracle://alice@localhost:1521?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:"},
			want: []string{
				"oracle://alice@localhost:1521/",
				"oracle://alice@localhost:1521?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:1521"},
			want: []string{
				"oracle://alice@localhost:1521/",
				"oracle://alice@localhost:1521?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:1521/"},
			want: []string{
				"oracle://alice@localhost:1521/service",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:1521/orcl"},
			want: []string{
				"oracle://alice@localhost:1521/orcl?",
			},
			wantResult: stdDirective,
		},
		{
			// gh743: a bare host with "?" should suggest conn-param keys,
			// not credential placeholders.
			args: []string{"oracle://localhost:1521?"},
			want: []string{
				"oracle://localhost:1521?CONNECTION+TIMEOUT=",
				"oracle://localhost:1521?SSL=",
				"oracle://localhost:1521?TRACE+FILE=",
				"oracle://localhost:1521?ssl=",
				"oracle://localhost:1521?wallet=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:1521/orcl?"},
			want: []string{
				"oracle://alice@localhost:1521/orcl?CONNECTION+TIMEOUT=",
				"oracle://alice@localhost:1521/orcl?SSL=",
				"oracle://alice@localhost:1521/orcl?TRACE+FILE=",
				"oracle://alice@localhost:1521/orcl?ssl=",
				"oracle://alice@localhost:1521/orcl?wallet=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"oracle://alice@localhost:1521/orcl?SSL="},
			want: []string{
				"oracle://alice@localhost:1521/orcl?SSL=false",
				"oracle://alice@localhost:1521/orcl?SSL=true",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, nil, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_History_Postgres(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)
	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	tr.Add(
		source.Source{
			Handle:   "@src1",
			Type:     drivertype.Pg,
			Location: "postgres://alice:abc123@dev.acme.com:7777/sakila?application_name=app1&channel_binding=prefer",
		},
		source.Source{
			Handle:   "@src2",
			Type:     drivertype.Pg,
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
				"postgres://alice@localhost?",
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
				"postgres://alice@dev.acme.com:7777/sakila?application_name=app1&channel_binding=prefer",
				"postgres://alice@prod.acme.com:8888/sales?application_name=app2&channel_binding=require",
				"postgres://alice@dev.acme.com:7777/",
				"postgres://alice@dev.acme.com:7777?",
				"postgres://alice@prod.acme.com:8888/",
				"postgres://alice@prod.acme.com:8888?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev"},
			want: []string{
				"postgres://alice@dev/",
				"postgres://alice@dev?",
				"postgres://alice@dev:5432/",
				"postgres://alice@dev:5432?",
				"postgres://alice@dev.acme.com:7777/sakila?application_name=app1&channel_binding=prefer",
				"postgres://alice@dev.acme.com:7777/",
				"postgres://alice@dev.acme.com:7777?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"postgres://alice@dev.acme.com"},
			want: []string{
				"postgres://alice@dev.acme.com/",
				"postgres://alice@dev.acme.com?",
				"postgres://alice@dev.acme.com:5432/",
				"postgres://alice@dev.acme.com:5432?",
				"postgres://alice@dev.acme.com:7777/sakila?application_name=app1&channel_binding=prefer",
				"postgres://alice@dev.acme.com:7777/",
				"postgres://alice@dev.acme.com:7777?",
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
				"postgres://alice@dev.acme.com/sakila?application_name=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_History_SQLServer(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)
	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	tr.Add(
		source.Source{
			Handle:   "@src1",
			Type:     drivertype.MSSQL,
			Location: "sqlserver://alice:abc123@dev.acme.com:7777?database=sakila&app+name=app1&encrypt=disable",
		},
		source.Source{
			Handle:   "@src2",
			Type:     drivertype.MSSQL,
			Location: "sqlserver://bob:abc123@prod.acme.com:8888?database=sales&app+name=app2&encrypt=true",
		},
		source.Source{
			Handle:   "@src3",
			Type:     drivertype.MSSQL,
			Location: "sqlserver://bob:abc123@prod.acme.com:8888/my_instance?database=sakila",
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
				"sqlserver://alice@localhost/",
				"sqlserver://alice@localhost?",
				"sqlserver://alice@localhost:1433/",
				"sqlserver://alice@localhost:1433?",
				"sqlserver://alice@dev.acme.com:7777?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@prod.acme.com:8888/my_instance?database=sakila",
				"sqlserver://alice@prod.acme.com:8888?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@dev.acme.com:7777/",
				"sqlserver://alice@dev.acme.com:7777?",
				"sqlserver://alice@prod.acme.com:8888/",
				"sqlserver://alice@prod.acme.com:8888?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev"},
			want: []string{
				"sqlserver://alice@dev/",
				"sqlserver://alice@dev?",
				"sqlserver://alice@dev:1433/",
				"sqlserver://alice@dev:1433?",
				"sqlserver://alice@dev.acme.com:7777?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com:7777/",
				"sqlserver://alice@dev.acme.com:7777?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@prod"},
			want: []string{
				"sqlserver://alice@prod/",
				"sqlserver://alice@prod?",
				"sqlserver://alice@prod:1433/",
				"sqlserver://alice@prod:1433?",
				"sqlserver://alice@prod.acme.com:8888/my_instance?database=sakila",
				"sqlserver://alice@prod.acme.com:8888?database=sales&app+name=app2&encrypt=true",
				"sqlserver://alice@prod.acme.com:8888/",
				"sqlserver://alice@prod.acme.com:8888?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com"},
			want: []string{
				"sqlserver://alice@dev.acme.com/",
				"sqlserver://alice@dev.acme.com?",
				"sqlserver://alice@dev.acme.com:1433/",
				"sqlserver://alice@dev.acme.com:1433?",
				"sqlserver://alice@dev.acme.com:7777?database=sakila&app+name=app1&encrypt=disable",
				"sqlserver://alice@dev.acme.com:7777/",
				"sqlserver://alice@dev.acme.com:7777?",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?"},
			want: []string{
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
				"sqlserver://alice@dev.acme.com?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database="},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sa"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sa&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=saki"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=saki&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sakila"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&",
			},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlserver://alice@dev.acme.com?database=sakila&app"},
			want: []string{
				"sqlserver://alice@dev.acme.com?database=sakila&app+name=",
			},
			wantResult: stdDirective,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

func TestCompleteAddLocation_History_SQLite3(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)
	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)
	src3Loc := "sqlite3://" + wd + "/my.db?cache=FAST"

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle: "@src2",
			Type:   drivertype.SQLite,
			// Note that this file doesn't actually exist
			Location: "sqlite3:///zz_dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
		},
		source.Source{
			Handle: "@src1",
			Type:   drivertype.SQLite,
			// Note that this file doesn't actually exist
			Location: "sqlite3:///zz_dir1/sqtest/sq/src1.db",
		},
		source.Source{
			Handle: "@src3",
			Type:   drivertype.SQLite,
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
				"sqlite3://data/",
				"sqlite3://my/",
				"sqlite3://my.db",
				"sqlite3://post/",
				"sqlite3://post.db",
				"sqlite3://sqlite/",
				"sqlite3://sqlite.db",
				src3Loc,
				"sqlite3:///zz_dir1/sqtest/sq/src1.db",
				"sqlite3:///zz_dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
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
			args: []string{"sqlite3:///zz_dir1/sqtest/"},
			want: []string{
				"sqlite3:///zz_dir1/sqtest/sq/src1.db",
				"sqlite3:///zz_dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3:///zz_dir1/sqtest/sq/not_a_dir"},
			want:       []string{},
			wantResult: stdDirective,
		},
		{
			args: []string{"sqlite3:///zz_dir1/sqtest/sq/src"},
			want: []string{
				"sqlite3:///zz_dir1/sqtest/sq/src1.db",
				"sqlite3:///zz_dir1/sqtest/sq/src2.db?mode=rwc&cache=FAST",
			},
			wantResult: stdDirective,
		},
		{
			args:       []string{"sqlite3:///zz_dir1/sqtest/sq/src1.db"},
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
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			require.Equal(t, tc.wantResult, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
		})
	}
}

// TestCompleteAddLocation_History_SecretsStripped verifies gh784:
// prior source locations are stripped of inline secrets (userinfo
// passwords and secret-bearing query parameter values) before they
// are offered as shell-completion candidates.
func TestCompleteAddLocation_History_SecretsStripped(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)
	wd := tu.Chdir(t, filepath.Join("testdata", "add_location"))
	t.Logf("Working dir: %s", wd)

	const (
		userinfoSecret = "userinfoSecret77"
		querySecret    = "querySecret88"
	)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle: "@pg1",
			Type:   drivertype.Pg,
			Location: "postgres://alice:" + userinfoSecret +
				"@dev.acme.com:7777/sakila?sslmode=require&sslpassword=" + querySecret,
		},
		source.Source{
			Handle: "@pg2",
			Type:   drivertype.Pg,
			// A ${scheme:path} placeholder password isn't itself a secret
			// (it's the text stored in config). net/url rejects the
			// placeholder characters in userinfo, but the parse gate
			// tolerates a placeholder password, so this source must still
			// contribute suggestions (bob, prod.acme.com:5432, the tail).
			Location: "postgres://bob:${keyring:pg-prod}@prod.acme.com:5432/sales?sslmode=verify-full",
		},
		source.Source{
			Handle: "@sl1",
			Type:   drivertype.SQLite,
			// Note that this file doesn't actually exist.
			Location: "sqlite3:///zz_dir1/sqtest/sq/app.db?_auth_user=admin&_auth_pass=" + querySecret,
		},
		source.Source{
			Handle: "@sl2",
			Type:   drivertype.SQLite,
			// Placeholder-valued secret query param: offered verbatim.
			Location: "sqlite3:///zz_dir1/sqtest/sq/app2.db?_auth_user=admin&_auth_pass=${keyring:sl-dev}",
		},
	)

	testCases := []struct {
		args []string
		want []string
	}{
		{
			args: []string{"sqlite3:///zz_dir1/sqtest/"},
			want: []string{
				"sqlite3:///zz_dir1/sqtest/sq/app.db?_auth_user=admin&_auth_pass=",
				"sqlite3:///zz_dir1/sqtest/sq/app2.db?_auth_user=admin&_auth_pass=${keyring:sl-dev}",
			},
		},
		{
			args: []string{"postgres://"},
			want: []string{
				"postgres://alice",
				"postgres://bob",
				"postgres://username",
			},
		},
		{
			args: []string{"postgres://alice@"},
			want: []string{
				"postgres://alice@localhost/",
				"postgres://alice@localhost?",
				"postgres://alice@localhost:5432/",
				"postgres://alice@localhost:5432?",
				"postgres://alice@dev.acme.com:7777/sakila?sslmode=require&sslpassword=",
				"postgres://alice@prod.acme.com:5432/sales?sslmode=verify-full",
				"postgres://alice@dev.acme.com:7777/",
				"postgres://alice@dev.acme.com:7777?",
				"postgres://alice@prod.acme.com:5432/",
				"postgres://alice@prod.acme.com:5432?",
			},
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, strings.Join(tc.args, "_")), func(t *testing.T) {
			args := append([]string{"add"}, tc.args...)
			got := testComplete(t, tr, args...)
			require.Equal(t, stdDirective, got.result, got.directives)
			require.Equal(t, tc.want, got.values)
			for _, v := range got.values {
				require.NotContains(t, v, userinfoSecret,
					"completion candidate must not contain the userinfo password")
				require.NotContains(t, v, querySecret,
					"completion candidate must not contain a secret query param value")
			}
		})
	}
}

// TestParseLoc_stage is no more: the legacy parsedLoc / plocStage
// parser was replaced by driver.LocationShape + driver.Walk. Stage
// detection is now covered by TestWalk in libsq/driver.

func TestDoCompleteAddLocationFile(t *testing.T) {
	tu.SkipIssueWindows(t, tu.GH372ShellCompletionWin)

	absDir := tu.Chdir(t, filepath.Join("testdata", "add_location"))
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
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			ctx := lg.NewContext(context.Background(), lgt.New(t))
			t.Logf("input: %s", tc.in)
			t.Logf("want:  %s", tc.want)
			got := cli.DoCompleteAddLocationFile(ctx, tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsDefiniteFilePath(t *testing.T) {
	testCases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"postgres://", false},
		{"sqlite3://./foo.db", false},
		{"file://", false},
		{"foo", false},
		{".", true},
		{"./foo.db", true},
		{"..", true},
		{"../sibling.db", true},
		{"~", true},
		{"~/data.db", true},
		{"/", true},
		{"/tmp/foo.db", true},
		{`\\server\share\db.sqlite`, true}, // Windows UNC
	}
	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, cli.IsDefiniteFilePath(tc.in))
		})
	}

	// filepath.IsAbs returns true for "C:\foo" on Windows only, so
	// the drive-letter case is gated to GOOS=windows. Without the
	// gate, this case would fail on Linux/macOS CI.
	if runtime.GOOS == "windows" {
		t.Run(`C:\foo`, func(t *testing.T) {
			require.True(t, cli.IsDefiniteFilePath(`C:\foo`))
		})
	}
}
