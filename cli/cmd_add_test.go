package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

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
			// duckdb with scheme
			loc:        "duckdb://" + proj.Abs(sakila.PathDuck),
			wantHandle: "@sakila",
			wantType:   drivertype.DuckDB,
		},
		{
			// duckdb without scheme, absolute path
			loc:        proj.Abs(sakila.PathDuck),
			wantHandle: "@sakila",
			wantType:   drivertype.DuckDB,
		},
		{
			// duckdb without scheme, relative path
			loc:        proj.Rel(sakila.PathDuck),
			wantHandle: "@sakila",
			wantType:   drivertype.DuckDB,
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

// TestCmdAdd_Keyring verifies that --keyring stores the password in the OS
// keyring mock and writes a ${keyring:...} placeholder into the source location.
func TestCmdAdd_Keyring(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@sakila_kr"
	const password = "hunter2"

	err := tr.Exec("add",
		"postgres://alice:"+password+"@localhost:5432/sakila",
		"--handle", handle, "--keyring",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Contains(t, src.Location, "${keyring:"+handle+"/password}")
	require.NotContains(t, src.Location, password)

	got, err := gokeyring.Get("sq", handle+"/password")
	require.NoError(t, err)
	require.Equal(t, password, got)
}

// TestCmdAdd_InlinePassword_OverridesDefault verifies that --inline-password
// forces inline storage even when secrets.default is set to "keyring".
func TestCmdAdd_InlinePassword_OverridesDefault(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Config.Options["secrets.default"] = "keyring"

	const handle = "@sakila_inline"
	err := tr.Exec("add",
		"postgres://alice:hunter2@localhost:5432/sakila",
		"--handle", handle, "--inline-password",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Contains(t, src.Location, "hunter2")
	require.NotContains(t, src.Location, "${keyring:")
}

// TestCmdAdd_DefaultKeyring_FromConfig verifies that when secrets.default is
// "keyring" and neither --keyring nor --inline-password is passed, the keyring
// path is taken.
func TestCmdAdd_DefaultKeyring_FromConfig(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Config.Options["secrets.default"] = "keyring"

	const handle = "@sakila_default_kr"
	err := tr.Exec("add",
		"postgres://alice:hunter2@localhost:5432/sakila",
		"--handle", handle,
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Contains(t, src.Location, "${keyring:"+handle+"/password}")
}

// TestCmdAdd_MutuallyExclusive verifies that --keyring and --inline-password
// cannot be used together.
func TestCmdAdd_MutuallyExclusive(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("add",
		"postgres://alice:hunter2@localhost:5432/sakila",
		"--handle", "@x", "--keyring", "--inline-password",
		"--driver", "postgres", "--skip-verify")
	require.Error(t, err)
}

// TestCmdAdd_Keyring_PromptsWhenNoPassword verifies that when --keyring is set
// but the URL has no inline password, sq reads the password from stdin (-p flag).
func TestCmdAdd_Keyring_PromptsWhenNoPassword(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Simulate piped stdin: write the password to a temp file and set it as Stdin.
	f, err := os.CreateTemp(t.TempDir(), "passwd")
	require.NoError(t, err)
	_, err = f.WriteString("piped-password\n")
	require.NoError(t, err)
	_, err = f.Seek(0, 0)
	require.NoError(t, err)
	tr.Run.Stdin = f

	const handle = "@sakila_kr_prompt"
	err = tr.Exec("add",
		"postgres://alice@localhost:5432/sakila", // no inline password
		"--handle", handle, "--keyring", "--password",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Contains(t, src.Location, "${keyring:"+handle+"/password}")

	got, err := gokeyring.Get("sq", handle+"/password")
	require.NoError(t, err)
	require.Equal(t, "piped-password", got)
}
