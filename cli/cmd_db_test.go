package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

// TestCmdDB_Placeholder_Resolved verifies that the db dump/restore/exec
// commands resolve ${scheme:path} placeholders in the source location
// before constructing the db-native tool command. These commands build
// pg_dump/pg_dumpall/pg_restore/psql invocations directly from
// src.Location, bypassing Grips.doOpen (which performs resolution for
// the query path), so they must resolve at the call site. The --print
// flag exposes the constructed command without needing a live server.
//
// See: https://github.com/neilotoole/sq/issues/783.
func TestCmdDB_Placeholder_Resolved(t *testing.T) {
	const (
		handle = "@pg_ph"
		envar  = "SQ_TEST_PG_PW"
		pw     = "hunter2"
	)

	testCases := []struct {
		name string
		args []string
	}{
		{name: "dump_catalog", args: []string{"db", "dump", "catalog", handle, "--print"}},
		{name: "dump_cluster", args: []string{"db", "dump", "cluster", handle, "--print"}},
		{name: "restore_catalog", args: []string{"db", "restore", "catalog", handle, "--print"}},
		{name: "restore_cluster", args: []string{"db", "restore", "cluster", handle, "--print"}},
		{name: "exec", args: []string{"db", "exec", handle, "--command", "SELECT 1", "--print"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envar, pw)
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			tr.Add(source.Source{
				Handle:   handle,
				Type:     drivertype.Pg,
				Location: "postgres://alice:${env:" + envar + "}@db.acme.com:5432/sakila",
			})

			require.NoError(t, tr.Exec(tc.args...))
			got := tr.OutString()
			require.Contains(t, got, pw,
				"printed cmd should contain the resolved secret")
			require.NotContains(t, got, "${env:",
				"printed cmd should not contain the unresolved placeholder")
		})
	}
}
