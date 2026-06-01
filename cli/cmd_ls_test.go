package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
)

// TestBug520_LsShowsPassword tests https://github.com/neilotoole/sq/issues/520.
func TestBug520_LsShowsPassword(t *testing.T) {
	t.Parallel()

	const password = "p_ssW0rd"

	testCases := []struct {
		name string
		loc  string
	}{
		{
			name: "sqlserver: no database param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost",
		},
		{
			name: "sqlserver: with database param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost?database=test",
		},
		{
			name: "sqlserver: no database param, with other param",
			loc:  "sqlserver://sa:p_ssW0rd@localhost?encrypt=true",
		},
		{
			name: "postgres: for regression/sanity check",
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			name: "mysql: for regression/sanity check",
			loc:  "mysql://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			name: "oracle: user/pass@host/service",
			loc:  "oracle://sakila:p_ssW0rd@localhost:1521/ORCL",
		},
		// NOTE: SQLite is not included because it uses file paths, not URLs with passwords.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := testrun.New(t.Context(), t, nil)

			require.NoError(t, tr.Exec("add", tc.loc, "--skip-verify"))
			require.NotContains(t, tr.OutString(), password, "should not print password")

			require.NoError(t, tr.Reset().Exec("ls"))
			require.NotContains(t, tr.OutString(), password, "should not print password")

			require.NoError(t, tr.Reset().Exec("ls", "-v", "--no-redact"))
			require.Contains(t, tr.OutString(), password, "should print password with --no-redact")

			require.NoError(t, tr.Reset().Exec("ls", "-v", "--reveal"))
			require.Contains(t, tr.OutString(), password, "should print password with --reveal")
		})
	}
}

// TestRedactFlags_Union verifies that --reveal and --no-redact behave
// as a union: setting either flips redaction off, setting both is not
// treated as a conflict, and the default (neither set) still redacts.
// The "default" case is the baseline that makes the disclosure-case
// assertions meaningful — without it, a regression that globally
// breaks redaction would silently make every union case pass.
func TestRedactFlags_Union(t *testing.T) {
	t.Parallel()

	const (
		loc      = "postgres://sakila:p_ssW0rd@localhost/sakila"
		password = "p_ssW0rd"
	)

	cases := []struct {
		name    string
		args    []string
		expects bool // whether the password should appear in output
	}{
		{name: "default", args: []string{"ls", "-v"}, expects: false},
		{name: "reveal", args: []string{"ls", "-v", "--reveal"}, expects: true},
		{name: "no-redact", args: []string{"ls", "-v", "--no-redact"}, expects: true},
		{name: "both", args: []string{"ls", "-v", "--reveal", "--no-redact"}, expects: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := testrun.New(t.Context(), t, nil)
			require.NoError(t, tr.Exec("add", loc, "--skip-verify"))
			require.NoError(t, tr.Reset().Exec(tc.args...))
			if tc.expects {
				require.Contains(t, tr.OutString(), password,
					"%s must flip redaction off", tc.name)
			} else {
				require.NotContains(t, tr.OutString(), password,
					"%s must redact by default", tc.name)
			}
		})
	}
}
