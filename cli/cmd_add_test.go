package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

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
	t.Parallel()

	th := testh.New(t)

	testCases := []struct {
		loc        string // first arg to "add" cmd
		driver     string // --driver flag
		handle     string // --handle flag
		wantHandle string
		wantType   source.DriverType
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
		{loc: proj.Abs(sakila.PathCSVActor), wantHandle: "@actor", wantType: csv.TypeCSV},
		{loc: proj.Abs(sakila.PathCSVActor), driver: "csv", wantHandle: "@actor", wantType: csv.TypeCSV},
		{loc: proj.Abs(sakila.PathCSVActor), driver: "xlsx", wantErr: true},
		// sqlite can be added both with and without the scheme "sqlite://"
		{
			loc: "sqlite3://" + proj.Abs(sakila.PathSL3), wantHandle: "@sakila",
			wantType: sqlite3.Type,
		}, // with scheme
		{
			loc: proj.Abs(sakila.PathSL3), wantHandle: "@sakila",
			wantType: sqlite3.Type,
		}, // without scheme, abs path
		{
			loc: proj.Rel(sakila.PathSL3), wantHandle: "@sakila",
			wantType: sqlite3.Type,
		}, // without scheme, relative path
		{loc: th.Source(sakila.Pg).Location, wantHandle: "@sakila", wantType: postgres.Type},
		{loc: th.Source(sakila.MS).Location, wantHandle: "@sakila", wantType: sqlserver.Type},
		{loc: th.Source(sakila.My).Location, wantHandle: "@sakila", wantType: mysql.Type},
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.StdinHandle, wantErr: true},   // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.ActiveHandle, wantErr: true},  // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.ScratchHandle, wantErr: true}, // reserved handle
		{loc: proj.Abs(sakila.PathCSVActor), handle: source.JoinHandle, wantErr: true},    // reserved handle
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tutil.Name(tc.wantHandle, tc.loc, tc.driver), func(t *testing.T) {
			t.Parallel()

			args := []string{"add", tc.loc}
			if tc.handle != "" {
				args = append(args, "--handle="+tc.handle)
			}
			if tc.driver != "" {
				args = append(args, "--driver="+tc.driver)
			}

			ru := newRun(t, nil)
			err := ru.Exec(args...)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify that the src was actually added
			gotSrc, err := ru.rc.Config.Collection.Get(tc.wantHandle)
			require.NoError(t, err)
			require.Equal(t, tc.wantHandle, gotSrc.Handle)
			require.Equal(t, tc.wantType, gotSrc.Type)
		})
	}
}

// TestCmdAdd_SQLite_Path has additional tests for sqlite paths.
func TestCmdAdd_SQLite_Path(t *testing.T) {
	t.Parallel()

	const h1 = `@s1`

	ru := newRun(t, nil)
	require.NoError(t, ru.Exec("add", "-j", "sqlite3://test.db", "-h", h1))
	got := ru.BindMap()

	absPath, err := filepath.Abs("test.db")
	require.NoError(t, err)
	absPath = filepath.ToSlash(absPath)

	wantLoc := "sqlite3://" + absPath
	require.Equal(t, wantLoc, got["location"])
}

func TestCmdAdd_Active(t *testing.T) {
	t.Parallel()

	const h1, h2, h3, h4 = "@h1", "@h2", "@h3", "@h4"

	// Verify that initially there are no sources.
	ru := newRun(t, nil)
	require.NoError(t, ru.Exec("ls"))
	require.Zero(t, ru.out.Len())

	// Add a new source. It should become the active source.
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("add", proj.Abs(sakila.PathCSVActor), "-h", h1))
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("src", "-j"))
	m := ru.BindMap()
	require.Equal(t, h1, m["handle"])

	// Add a second src, without the --active flag. The active src
	// should remain h1.
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("add", proj.Abs(sakila.PathCSVActor), "-h", h2))
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("src", "-j"))
	m = ru.BindMap()
	require.Equal(t, h1, m["handle"], "active source should still be %s", h1)

	// Add a third src, this time with the --active flag. The active src
	// should become h3.
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("add", proj.Abs(sakila.PathCSVActor), "-h", h3, "--active"))
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("src", "-j"))
	m = ru.BindMap()
	require.Equal(t, h3, m["handle"], "active source now be %s", h3)

	// Same again with a fourth src, but this time using the shorthand -a flag.
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("add", proj.Abs(sakila.PathCSVActor), "-h", h4, "-a"))
	ru = newRun(t, ru)
	require.NoError(t, ru.Exec("src", "-j"))
	m = ru.BindMap()
	require.Equal(t, h4, m["handle"], "active source now be %s", h4)
}
