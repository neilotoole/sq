package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
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

// keyringPlaceholderRE matches a Location set by `--store keyring`:
// exactly "${keyring:<id>}" where <id> is 10 chars from the Crockford
// alphabet (excludes i, l, o, u).
var keyringPlaceholderRE = regexp.MustCompile(`^\$\{keyring:([0-9a-hjkmnp-tv-z]{10})\}$`)

// extractKeyringID returns the opaque ID from a "${keyring:<id>}" Location.
// Fails the test if loc is not in the expected form.
func extractKeyringID(t *testing.T, loc string) string {
	t.Helper()
	m := keyringPlaceholderRE.FindStringSubmatch(loc)
	require.Len(t, m, 2, "Location %q is not a bare ${keyring:<id>} placeholder", loc)
	return m[1]
}

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

// TestCmdAdd_Keyring verifies that --keyring stores the full DSN in the
// OS keyring at a fresh opaque ID and writes a bare ${keyring:<id>}
// placeholder into the source Location.
func TestCmdAdd_Keyring(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@sakila_kr"
	const password = "hunter2"
	const dsn = "postgres://alice:" + password + "@localhost:5432/sakila"

	err := tr.Exec("add", dsn,
		"--handle", handle, "--store", "keyring",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)

	// Location is a bare placeholder; no part of the URL leaks into YAML.
	id := extractKeyringID(t, src.Location)
	require.NotContains(t, src.Location, password)
	require.NotContains(t, src.Location, "alice")
	require.NotContains(t, src.Location, "localhost")

	// Keyring holds the entire DSN, not just the password.
	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, dsn, got)
}

// TestCmdAdd_StoreInline_OverridesConfig verifies that --store=inline
// forces inline storage even when secrets.store is set to "keyring".
func TestCmdAdd_StoreInline_OverridesConfig(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Config.Options["secrets.store"] = "keyring"

	const handle = "@sakila_inline"
	err := tr.Exec("add",
		"postgres://alice:hunter2@localhost:5432/sakila",
		"--handle", handle, "--store", "inline",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Contains(t, src.Location, "hunter2")
	require.NotContains(t, src.Location, "${keyring:")
}

// TestCmdAdd_StoreKeyring_FromConfig verifies that when secrets.store
// is "keyring" and no --store flag is passed, the keyring path is
// taken — full DSN in keyring, bare placeholder in Location.
func TestCmdAdd_StoreKeyring_FromConfig(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Config.Options["secrets.store"] = "keyring"

	const handle = "@sakila_default_kr"
	const dsn = "postgres://alice:hunter2@localhost:5432/sakila"
	err := tr.Exec("add", dsn,
		"--handle", handle,
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	id := extractKeyringID(t, src.Location)

	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, dsn, got)
}

// TestCmdAdd_StoreInvalidValue verifies that --store rejects unknown
// values with an actionable error. Replaces the prior
// TestCmdAdd_MutuallyExclusive test which is now moot: a single
// string flag can't be mutually exclusive with itself.
func TestCmdAdd_StoreInvalidValue(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("add",
		"postgres://alice:hunter2@localhost:5432/sakila",
		"--handle", "@x", "--store", "bogus",
		"--driver", "postgres", "--skip-verify")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--store",
		"error should name the offending flag")
	require.Contains(t, err.Error(), "inline",
		"error should list valid values")
	require.Contains(t, err.Error(), "keyring",
		"error should list valid values")
}

// TestCmdAdd_Placeholder_FileRelativeIsAbsolutized verifies that a
// bare-relative path inside a ${file:...} placeholder is captured
// against the current working directory at sq-add time, so the
// persisted Location is independent of where sq is later invoked
// from. Mirrors the user-reported friction case
// (`sq add '${file:./pg.dsn}'`).
func TestCmdAdd_Placeholder_FileRelativeIsAbsolutized(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pg.dsn"),
		[]byte("postgres://alice:hunter2@localhost:5432/sakila\n"),
		0o600))

	// Run sq add from inside the temp dir so the relative path resolves
	// against it. Snapshot the real cwd post-chdir so the assertion
	// matches what filepath.Abs will produce — on macOS, /tmp is a
	// symlink to /private/tmp and t.TempDir returns the unresolved form.
	pwd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(pwd) })
	require.NoError(t, os.Chdir(dir))
	resolvedDir, err := os.Getwd()
	require.NoError(t, err)

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@from_relative_file"
	require.NoError(t, tr.Exec("add", "${file:./pg.dsn}",
		"--handle", handle, "--skip-verify"))

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	wantLoc := "${file:" + filepath.Join(resolvedDir, "pg.dsn") + "}"
	require.Equal(t, wantLoc, src.Location,
		"./pg.dsn should have been absolutized at add time")
}

// TestCmdAdd_Placeholder_FilePassthroughForms verifies that path
// forms the file resolver already accepts (absolute, ~/) are
// preserved verbatim — absolutizing them would harm portability
// (~/ is user-relative by design) or be pointless (absolute paths).
func TestCmdAdd_Placeholder_FilePassthroughForms(t *testing.T) {
	tests := []struct {
		name string
		loc  string
	}{
		{name: "absolute", loc: "${file:/etc/sq/pg.dsn}"},
		{name: "home-relative", loc: "${file:~/.sq/pg.dsn}"},
		{name: "file URI sugar", loc: "${file:///etc/sq/pg.dsn}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			const handle = "@passthrough"
			require.NoError(t, tr.Exec("add", tc.loc,
				"--handle", handle,
				"--driver", "postgres", // skip add-time resolution
				"--skip-verify"))
			src, err := tr.Run.Config.Collection.Get(handle)
			require.NoError(t, err)
			require.Equal(t, tc.loc, src.Location,
				"absolute/~ /file:/// paths must pass through unchanged")
		})
	}
}

// TestCmdAdd_Placeholder_NonFileSchemeUntouched verifies that the
// add-time rewriter is scoped to the file scheme. A relative-looking
// body inside ${env:...} or ${keyring:...} is opaque to sq and must
// not be rewritten — the env-var or keyring-id semantics are unrelated
// to filesystem paths.
func TestCmdAdd_Placeholder_NonFileSchemeUntouched(t *testing.T) {
	t.Setenv("SQ_TEST_DSN_PT", "postgres://alice:hunter2@localhost:5432/sakila")
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@env_relative_looking"
	// "./SQ_TEST_DSN_PT" is what would happen if env-var names that
	// happen to contain "./" were absolutized. They must not be.
	require.NoError(t, tr.Exec("add", "${env:SQ_TEST_DSN_PT}",
		"--handle", handle, "--skip-verify"))
	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Equal(t, "${env:SQ_TEST_DSN_PT}", src.Location)
}

// TestCmdAdd_StoreKeyring_RequiresPassword verifies that --store=keyring
// rejects an invocation with no password — neither inline in the URL
// nor via -p. Without this guard, sq would silently produce a keyring
// entry holding an incomplete DSN; the error tells the user how to fix it.
func TestCmdAdd_StoreKeyring_RequiresPassword(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add",
		"postgres://alice@localhost:5432/sakila", // no inline password
		"--handle", "@needs_pw", "--store", "keyring",
		"--driver", "postgres", "--skip-verify")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--store",
		"error should name the offending flag")
	require.Contains(t, err.Error(), "keyring",
		"error should name the offending --store value")
	require.Contains(t, err.Error(), "--password",
		"error should point at -p / --password as the recovery path")
}

// TestCmdAdd_StoreKeyring_RequiresURL verifies that --store=keyring
// rejects a non-URL Location. File paths and similar have nothing
// useful to store in keyring; allowing the flag would create an orphan
// entry with a nonsensical value.
func TestCmdAdd_StoreKeyring_RequiresURL(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	csv := filepath.Join(t.TempDir(), "actor.csv")
	require.NoError(t, os.WriteFile(csv, []byte("a,b\n1,2\n"), 0o600))

	err := tr.Exec("add", csv,
		"--handle", "@not_url", "--store", "keyring",
		"--driver", "csv", "--skip-verify")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--store",
		"error should name the offending flag")
	require.Contains(t, err.Error(), "keyring",
		"error should name the offending --store value")
	require.Contains(t, err.Error(), "URL",
		"error should clarify a URL location is required")
}

// TestCmdAdd_Placeholder_AutoResolve_Env verifies that a bare placeholder
// Location triggers add-time resolution to infer the driver. The
// placeholder itself (not the resolved value) is what lands in YAML.
func TestCmdAdd_Placeholder_AutoResolve_Env(t *testing.T) {
	t.Setenv("SQ_TEST_DSN_FOR_ADD", "postgres://alice:hunter2@localhost:5432/sakila")

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@sakila_env"
	err := tr.Exec("add", "${env:SQ_TEST_DSN_FOR_ADD}",
		"--handle", handle, "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)

	// Location is the placeholder verbatim; the resolved DSN is not persisted.
	require.Equal(t, "${env:SQ_TEST_DSN_FOR_ADD}", src.Location)
	// Driver was inferred from the resolved URL's scheme.
	require.Equal(t, drivertype.Pg, src.Type)
}

// TestCmdAdd_Placeholder_ExplicitDriver verifies that --driver short-circuits
// add-time resolution. The Location need not even resolve at add time.
func TestCmdAdd_Placeholder_ExplicitDriver(t *testing.T) {
	// SQ_TEST_DSN_FOR_ADD_NONEXISTENT is deliberately unset; --driver
	// should make sq not attempt resolution at all.
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@sakila_explicit"
	err := tr.Exec("add", "${env:SQ_TEST_DSN_FOR_ADD_NONEXISTENT}",
		"--handle", handle,
		"--driver", "postgres",
		"--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Equal(t, "${env:SQ_TEST_DSN_FOR_ADD_NONEXISTENT}", src.Location)
	require.Equal(t, drivertype.Pg, src.Type)
}

// TestCmdAdd_Placeholder_ResolveFails verifies that when neither --driver
// nor --skip-verify is set and the placeholder cannot be resolved, the
// error surfaces the disambiguated recovery hint: --driver is the actual
// escape hatch, and --skip-verify alone is NOT sufficient (it only
// skips the post-add ping).
func TestCmdAdd_Placeholder_ResolveFails(t *testing.T) {
	// Variable is deliberately unset.
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add", "${env:SQ_TEST_DSN_FOR_ADD_MISSING}",
		"--handle", "@x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--driver",
		"error should hint at --driver as the recovery path")
	require.Contains(t, err.Error(), "--skip-verify",
		"error should clarify what --skip-verify does (only skips post-add ping)")
	require.Contains(t, err.Error(), "post-add ping",
		"error should explicitly say --skip-verify is not enough on its own")
}

// TestCmdAdd_Placeholder_SkipVerifyWithDriverWorks verifies that
// --skip-verify combined with --driver succeeds for a placeholder
// Location AND does NOT touch the resolver (the env var is
// intentionally absent — if sq tried to resolve, we'd see a
// different error mentioning the env scheme). The two flags are
// orthogonal: --driver suppresses the inference resolve, --skip-verify
// suppresses the post-add ping; both can be set independently.
func TestCmdAdd_Placeholder_SkipVerifyWithDriverWorks(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@skipverify_with_driver"
	require.NoError(t, tr.Exec("add", "${env:SQ_TEST_DSN_FOR_ADD_NEVER_SET}",
		"--handle", handle,
		"--driver", "postgres",
		"--skip-verify"))

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Equal(t, "${env:SQ_TEST_DSN_FOR_ADD_NEVER_SET}", src.Location)
	require.Equal(t, drivertype.Pg, src.Type)
}

// TestCmdAdd_Placeholder_StoreRejected verifies that --store with any
// value (keyring or inline) is incompatible with a placeholder Location:
// the value already lives in an external store, so sq has nothing to
// write.
func TestCmdAdd_Placeholder_StoreRejected(t *testing.T) {
	tests := []struct{ name, store string }{
		{name: "keyring", store: "keyring"},
		{name: "inline", store: "inline"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SQ_TEST_DSN_FOR_REJECTED_"+tc.name,
				"postgres://alice:hunter2@localhost/sakila")
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)

			err := tr.Exec("add", "${env:SQ_TEST_DSN_FOR_REJECTED_"+tc.name+"}",
				"--handle", "@x", "--store", tc.store,
				"--driver", "postgres", "--skip-verify")
			require.Error(t, err)
			require.Contains(t, err.Error(), "--store",
				"error should name the offending flag")
		})
	}
}

// TestCmdAdd_StoreKeyring_PromptsWhenNoPassword verifies that when
// --store=keyring is set with --password, sq reads the password from
// stdin and splices it into the URL before storing the full DSN in
// keyring.
func TestCmdAdd_StoreKeyring_PromptsWhenNoPassword(t *testing.T) {
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
		"--handle", handle, "--store", "keyring", "--password",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	id := extractKeyringID(t, src.Location)

	// The piped password was spliced into the URL; the keyring entry
	// holds the full DSN with the password in userinfo.
	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:piped-password@localhost:5432/sakila", got)
}
