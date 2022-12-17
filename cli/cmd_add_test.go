package cli_test

import (
	"github.com/neilotoole/sq/testh/tutil"
	"testing"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
)

func TestCmdAdd(t *testing.T) {
	th := testh.New(t)

	testCases := []struct {
		loc        string // first arg to "add" cmd
		driver     string // --driver flag
		handle     string // --handle flag
		wantHandle string
		wantType   source.Type
		wantErr    bool
	}{
		{loc: "", wantErr: true},
		{loc: "   ", wantErr: true},
		{loc: "/", wantErr: true},
		{loc: "../../", wantErr: true},
		{loc: "does/not/exist", wantErr: true},
		{loc: "_", wantErr: true},
		{loc: ".", wantErr: true},
		{loc: "/", wantErr: true},
		{loc: "../does/not/exist.csv", wantErr: true},
		{loc: proj.Rel(sakila.PathCSVActor), handle: "@h1", wantHandle: "@h1", wantType: csv.TypeCSV}, // relative path
		{loc: proj.Abs(sakila.PathCSVActor), handle: "@h1", wantHandle: "@h1", wantType: csv.TypeCSV}, // absolute path
		{loc: proj.Abs(sakila.PathCSVActor), wantHandle: "@actor_csv", wantType: csv.TypeCSV},
		{loc: proj.Abs(sakila.PathCSVActor), driver: "csv", wantHandle: "@actor_csv", wantType: csv.TypeCSV},
		{loc: proj.Abs(sakila.PathCSVActor), driver: "xlsx", wantErr: true},
		// sqlite can be added both with and without the scheme "sqlite://"
		{loc: "sqlite3://" + proj.Abs(sakila.PathSL3), wantHandle: "@sakila_sqlite", wantType: sqlite3.Type}, // with scheme
		{loc: proj.Abs(sakila.PathSL3), wantHandle: "@sakila_sqlite", wantType: sqlite3.Type},                // without scheme, abs path
		{loc: proj.Rel(sakila.PathSL3), wantHandle: "@sakila_sqlite", wantType: sqlite3.Type},                // without scheme, relative path
		{loc: th.Source(sakila.Pg).Location, wantHandle: "@sakila_pg", wantType: postgres.Type},
		{loc: th.Source(sakila.MS).Location, wantHandle: "@sakila_mssql", wantType: sqlserver.Type},
		{loc: th.Source(sakila.My).Location, wantHandle: "@sakila_my", wantType: mysql.Type},
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.StdinHandle, wantErr: true},   // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.ActiveHandle, wantErr: true},  // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.ScratchHandle, wantErr: true}, // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.JoinHandle, wantErr: true},    // reserved handle
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.wantHandle, tc.loc, tc.driver), func(t *testing.T) {
			args := []string{"add", tc.loc}
			if tc.handle != "" {
				args = append(args, "--handle="+tc.handle)
			}
			if tc.driver != "" {
				args = append(args, "--driver="+tc.driver)
			}

			ru := newRun(t)
			err := ru.exec(args...)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify that the src was actually added
			gotSrc, err := ru.rc.Config.Sources.Get(tc.wantHandle)
			require.NoError(t, err)
			require.Equal(t, tc.wantHandle, gotSrc.Handle)
			require.Equal(t, tc.wantType, gotSrc.Type)
		})
	}
}
