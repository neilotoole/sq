package cli_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/proj"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestCmdAdd(t *testing.T) {
	type query struct {
		// q is the SLQ query to execute
		q        string
		wantRows int
		wantCols int
	}

	actorDataQuery := &query{
		q:        ".data",
		wantRows: sakila.TblActorCount,
		wantCols: len(sakila.TblActorCols()),
	}
	_ = actorDataQuery

	testCases := []struct {
		// Set only one of loc, or locFromHandle, to create
		// the first arg to "add" cmd.
		//
		// loc, when set, will be used directly.
		loc string
		// locFromHandle, when set, gets the location from the
		// config source with the given handle.
		locFromHandle string

		driver       string // --driver flag
		handle       string // --handle flag
		wantHandle   string
		wantType     drivertype.Type
		wantOptions  options.Options
		wantAddErr   bool
		wantQueryErr bool
		query        *query
	}{
		{
			loc:        "",
			wantAddErr: true,
		},
		{
			loc:        "   ",
			wantAddErr: true,
		},
		{
			loc:        "/",
			wantAddErr: true,
		},
		{
			loc:        "../../",
			wantAddErr: true,
		},
		{
			loc:        "does/not/exist",
			wantAddErr: true,
		},
		{
			loc:        "_",
			wantAddErr: true,
		},
		{
			loc:        ".",
			wantAddErr: true,
		},
		{
			loc:        "/",
			wantAddErr: true,
		},
		{
			loc:        "../does/not/exist.csv",
			wantAddErr: true,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     "@h1",
			wantHandle: "@h1",
			wantType:   drivertype.CSV,
			query:      actorDataQuery,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     "@h1",
			wantHandle: "@h1",
			wantType:   drivertype.CSV,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			wantHandle: "@actor",
			wantType:   drivertype.CSV,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			driver:     "csv",
			wantHandle: "@actor",
			wantType:   drivertype.CSV,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			driver:     "xlsx",
			wantHandle: "@actor",
			wantType:   drivertype.XLSX,
			// It's legal to add a CSV file with the xlsx driver.
			wantAddErr: false,
			// But it should fail when we try to query it.
			wantQueryErr: true,
		},
		{
			loc:        proj.Abs(sakila.PathTSVActor),
			handle:     "@h1",
			wantHandle: "@h1",
			wantType:   drivertype.TSV,
			query:      actorDataQuery,
		},
		{
			loc:        proj.Abs(sakila.PathTSVActorNoHeader),
			handle:     "@h1",
			wantHandle: "@h1",
			wantType:   drivertype.TSV,
			query:      actorDataQuery,
		},
		{
			// sqlite can be added both with and without the scheme "sqlite://"
			loc:        "sqlite3://" + proj.Abs(sakila.PathSL3),
			wantHandle: "@sakila",
			wantType:   drivertype.SQLite,
		},

		{
			// with scheme
			loc:        proj.Abs(sakila.PathSL3),
			wantHandle: "@sakila",
			wantType:   drivertype.SQLite,
		},

		{
			// without scheme, relative path
			loc:        proj.Rel(sakila.PathSL3),
			wantHandle: "@sakila",
			wantType:   drivertype.SQLite,
		},
		{
			locFromHandle: sakila.Pg,
			wantHandle:    "@sakila",
			wantType:      drivertype.Pg,
		},
		{
			locFromHandle: sakila.MS,
			wantHandle:    "@sakila",
			wantType:      drivertype.MSSQL,
		},
		{
			locFromHandle: sakila.My,
			wantHandle:    "@sakila",
			wantType:      drivertype.MySQL,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     source.StdinHandle, // reserved handle
			wantAddErr: true,
		},

		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     source.ActiveHandle, // reserved handle
			wantAddErr: true,
		},

		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     source.ScratchHandle, // reserved handle
			wantAddErr: true,
		},
		{
			loc:        proj.Abs(sakila.PathCSVActor),
			handle:     source.JoinHandle, // reserved handle
			wantAddErr: true,
		},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.wantHandle, tc.loc, tc.locFromHandle, tc.driver), func(t *testing.T) {
			if tc.locFromHandle != "" {
				th := testh.New(t)
				tc.loc = th.Source(tc.locFromHandle).Location
			}

			args := []string{"add", tc.loc}
			if tc.handle != "" {
				args = append(args, "--handle="+tc.handle)
			}
			if tc.driver != "" {
				args = append(args, "--driver="+tc.driver)
			}

			tr := testrun.New(context.Background(), t, nil)
			err := tr.Exec(args...)
			if tc.wantAddErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify that the src was actually added
			gotSrc, err := tr.Run.Config.Collection.Get(tc.wantHandle)
			require.NoError(t, err)
			require.Equal(t, tc.wantHandle, gotSrc.Handle)
			require.Equal(t, tc.wantType, gotSrc.Type)
			require.Equal(t, len(tc.wantOptions), len(gotSrc.Options))

			if tc.query == nil {
				return
			}

			err = tr.Reset().Exec(tc.query.q, "--json")
			if tc.wantQueryErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var results []map[string]any
			tr.Bind(&results)

			require.Equal(t, tc.query.wantRows, len(results))
			if tc.query.wantRows > 0 {
				require.Equal(t, tc.query.wantCols, len(results[0]))
			}
		})
	}
}

// TestCmdAdd_SQLite_Path has additional tests for sqlite paths.
func TestCmdAdd_SQLite_Path(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const h1 = `@s1`

	tr := testrun.New(ctx, t, nil)
	require.NoError(t, tr.Exec("add", "-j", "sqlite3://test.db", "--handle", h1))
	got := tr.BindMap()

	absPath, err := filepath.Abs("test.db")
	require.NoError(t, err)
	absPath = filepath.ToSlash(absPath)

	wantLoc := "sqlite3://" + absPath
	require.Equal(t, wantLoc, got["location"])
}

func TestCmdAdd_Active(t *testing.T) {
	t.Parallel()

	const h1, h2, h3, h4 = "@h1", "@h2", "@h3", "@h4"
	ctx := context.Background()

	// Verify that initially there are no sources.
	tr := testrun.New(ctx, t, nil)
	require.NoError(t, tr.Exec("ls"))
	require.Zero(t, tr.Out.Len())

	// Add a new source. It should become the active source.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", h1))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "-j"))
	m := tr.BindMap()
	require.Equal(t, h1, m["handle"])

	// Add a second src, without the --active flag. The active src
	// should remain h1.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", h2))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "-j"))
	m = tr.BindMap()
	require.Equal(t, h1, m["handle"], "active source should still be %s", h1)

	// Add a third src, this time with the --active flag. The active src
	// should become h3.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", h3, "--active"))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "-j"))
	m = tr.BindMap()
	require.Equal(t, h3, m["handle"], "active source now be %s", h3)

	// Same again with a fourth src, but this time using the shorthand -a flag.
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("add", proj.Abs(sakila.PathCSVActor), "--handle", h4, "-a"))
	tr = testrun.New(ctx, t, tr)
	require.NoError(t, tr.Exec("src", "-j"))
	m = tr.BindMap()
	require.Equal(t, h4, m["handle"], "active source now be %s", h4)
}
