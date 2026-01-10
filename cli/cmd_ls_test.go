package cli_test

import (
	"testing"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/stretchr/testify/require"
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
		})
	}
}
