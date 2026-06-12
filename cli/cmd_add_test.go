package cli_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
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

// TestCmdAdd_Keyring verifies that --store keyring stores the full DSN
// in the OS keyring at a fresh opaque ID and writes a bare
// ${keyring:<id>} placeholder into the source Location.
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

// TestCmdAdd_StoreKeyringConfig_PasswordlessFallsThrough verifies that
// when secrets.store=keyring is the config default (no explicit
// --store flag), a source without a password — file path, or URL
// without userinfo — adds successfully via the inline path instead of
// being rejected by applyKeyring's password requirement. The keyring
// default should only fire when there's actually a secret to store.
func TestCmdAdd_StoreKeyringConfig_PasswordlessFallsThrough(t *testing.T) {
	tests := []struct {
		name string
		loc  string
		typ  string
	}{
		{name: "file path", loc: "@INVENTORY@", typ: "csv"},
		{name: "url no password", loc: "postgres://alice@localhost:5432/sakila", typ: "postgres"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gokeyring.MockInit()
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			tr.Run.Config.Options["secrets.store"] = "keyring"

			loc := tc.loc
			if loc == "@INVENTORY@" {
				csv := filepath.Join(t.TempDir(), "actor.csv")
				require.NoError(t, os.WriteFile(csv, []byte("a,b\n1,2\n"), 0o600))
				loc = csv
			}

			const handle = "@passwordless"
			err := tr.Exec("add", loc,
				"--handle", handle,
				"--driver", tc.typ, "--skip-verify")
			require.NoError(t, err,
				"keyring config default must not reject passwordless adds")

			src, err := tr.Run.Config.Collection.Get(handle)
			require.NoError(t, err)
			require.NotContains(t, src.Location, "${keyring:",
				"no secret to keyring → Location should stay inline")
		})
	}
}

// TestCmdAdd_StoreKeyringExplicit_PasswordlessRejected verifies the
// inverse: when the user explicitly passes --store keyring on a
// passwordless source, the rejection still fires. Explicit user
// intent is honored.
func TestCmdAdd_StoreKeyringExplicit_PasswordlessRejected(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add", "postgres://alice@localhost:5432/sakila",
		"--handle", "@explicit_no_pw",
		"--store", "keyring",
		"--driver", "postgres", "--skip-verify")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--store",
		"explicit --store keyring without a password still rejects")
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

// TestCmdAdd_BareOpURI_IsAutoWrapped verifies the 1Password copy-paste
// shortcut: a bare "op://..." location (the literal form emitted by
// 1Password's "Copy Secret Reference" and by `op item get --format`)
// is treated as a sugar for the canonical "${op://...}" placeholder,
// so users don't have to add the ${} themselves.
func TestCmdAdd_BareOpURI_IsAutoWrapped(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// --driver short-circuits add-time resolution, so the test does
	// not need a live op CLI or stub.
	const handle = "@sakila_pg"
	err := tr.Exec("add", "op://Private/sakila_pg/dsn",
		"--handle", handle,
		"--driver", "postgres",
		"--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	// The location stored in YAML is the canonical wrapped form.
	require.Equal(t, "${op://Private/sakila_pg/dsn}", src.Location)
	require.Equal(t, drivertype.Pg, src.Type)
}

// TestCmdAdd_Placeholder_NonDSNResolved_GuidesUser verifies that when a
// placeholder's resolved value is a bare value (e.g. just a password,
// not a full DSN), the error names the placeholder, lists the three
// recovery paths, and never leaks the resolved value.
func TestCmdAdd_Placeholder_NonDSNResolved_GuidesUser(t *testing.T) {
	// Resolved value is a bare password, not a DSN. Field name is
	// deliberately "password" — the shape of the value is what matters
	// for inference, not the field name.
	t.Setenv("SQ_TEST_DSN_FOR_ADD_BARE_PW", "hunter2-secret-not-a-dsn")

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add", "${env:SQ_TEST_DSN_FOR_ADD_BARE_PW}", "--handle", "@x")
	require.Error(t, err)
	msg := err.Error()
	require.Contains(t, msg, "${env:SQ_TEST_DSN_FOR_ADD_BARE_PW}",
		"error should name the placeholder so the user knows which secret failed")
	require.NotContains(t, msg, "hunter2-secret-not-a-dsn",
		"error must not leak the resolved value")
	require.Contains(t, msg, "compose",
		"error should mention composition as a recovery path")
	require.Contains(t, msg, "--driver",
		"error should mention --driver as a recovery path")
	require.Contains(t, msg, "postgres://",
		"error should include a concrete DSN example")
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
	tr.PipeStdin("piped-password\n")

	const handle = "@sakila_kr_prompt"
	err := tr.Exec("add",
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

// TestCmdAdd_InlinePassword_EscapesDollar verifies that a prompted or
// piped password containing '$' is escaped ('$' -> '$$') when spliced
// inline into the stored location. The stored location is a
// placeholder template in which '$$' means a literal '$'; without
// escaping, the connect path's unescape would corrupt the literal
// password (e.g. 'pa$$word' -> 'pa$word').
func TestCmdAdd_InlinePassword_EscapesDollar(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.PipeStdin("pa$$word\n")

	const handle = "@sakila_inline_dollar"
	err := tr.Exec("add",
		"postgres://alice@localhost:5432/sakila",
		"--handle", handle, "--store", "inline", "--password",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:pa$$$$word@localhost:5432/sakila", src.Location,
		"literal password must be stored in escaped (template) form")

	// Round-trip: the driver must receive the literal password. Zero
	// refs, so no secret.Registry is needed on the context.
	resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:pa$$word@localhost:5432/sakila", resolved.Location)
}

// TestCmdAdd_KeyringPassword_NotEscaped verifies that the keyring path
// stores the literal (unescaped) DSN: keyring slots hold literal
// values that Registry.Expand splices raw at connect time, so the
// template escaping applied by the inline path must NOT happen here.
func TestCmdAdd_KeyringPassword_NotEscaped(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.PipeStdin("pa$$word\n")

	const handle = "@sakila_kr_dollar"
	err := tr.Exec("add",
		"postgres://alice@localhost:5432/sakila",
		"--handle", handle, "--store", "keyring", "--password",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	id := extractKeyringID(t, src.Location)

	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:pa$$word@localhost:5432/sakila", got,
		"keyring holds the literal DSN, no template escaping")
}

// TestCmdAdd_KeyringInlinePassword_Unescaped verifies that the
// keyring path converts the typed location (a placeholder template,
// where '$$' means a literal '$') to its literal form before storing:
// keyring slots hold literals that Registry.Expand splices raw at
// connect, so storing the template bytes verbatim would hand the
// driver a still-escaped credential.
func TestCmdAdd_KeyringInlinePassword_Unescaped(t *testing.T) {
	gokeyring.MockInit()

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	const handle = "@sakila_kr_inline_dollar"
	// Typed template form: '$$' means the literal password is 'p$ss'.
	err := tr.Exec("add",
		"postgres://alice:p$$ss@localhost:5432/sakila",
		"--handle", handle, "--store", "keyring",
		"--driver", "postgres", "--skip-verify")
	require.NoError(t, err)

	src, err := tr.Run.Config.Collection.Get(handle)
	require.NoError(t, err)
	id := extractKeyringID(t, src.Location)

	got, err := gokeyring.Get("sq", id)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:p$ss@localhost:5432/sakila", got,
		"keyring must hold the literal DSN, not the template bytes")
}

// TestCmdAdd_FileLocation_RawDollarDollar verifies that adding a file
// whose typed path contains '$$' fails at add time with a clear error
// when the file exists at the typed path but not at the
// template-interpreted path. Without this check, the add would fail at
// the ping (or, with --skip-verify, persist a broken source) with an
// error naming a path the user never typed, because the connect path
// interprets '$$' as an escaped '$'.
func TestCmdAdd_FileLocation_RawDollarDollar(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "data$$file.csv")
	require.NoError(t, os.WriteFile(fpath, []byte("a,b\n1,2\n"), 0o600))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add", fpath, "--handle", "@raw_dollar", "--skip-verify")
	require.Error(t, err,
		"raw '$$' path whose interpreted form doesn't exist must be rejected at add time")
	require.Contains(t, err.Error(), "$$")
}

// TestCmdAdd_FileLocation_EscapedDollarDollar verifies that the
// documented escaped form works end-to-end: the stored location is the
// typed template, detection and the add-time ping interpret it, and
// the source connects.
func TestCmdAdd_FileLocation_EscapedDollarDollar(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "data$$file.csv")
	require.NoError(t, os.WriteFile(fpath, []byte("a,b\n1,2\n"), 0o600))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	escaped := strings.ReplaceAll(fpath, "$", "$$")
	require.NoError(t, tr.Exec("add", escaped, "--handle", "@esc_dollar"),
		"escaped form must pass detection and the add-time ping")

	src, err := tr.Run.Config.Collection.Get("@esc_dollar")
	require.NoError(t, err)
	require.Equal(t, escaped, src.Location, "the typed template form is what gets stored")
}

// TestCmdAdd_SQLiteLocation_RawDollarDollar verifies that the '$$'
// add-time check also covers driver-prefixed file-DB locations. This
// matters doubly for SQLite: it CREATES missing files on open, so
// without the check, the add-time ping would silently create and open
// an empty DB at the interpreted path while the user's real database
// is never touched.
func TestCmdAdd_SQLiteLocation_RawDollarDollar(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "my$$file.db")
	require.NoError(t, os.WriteFile(fpath, []byte{}, 0o600))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("add", "sqlite3:"+fpath,
		"--handle", "@sl_raw_dollar", "--driver", "sqlite3", "--skip-verify")
	require.Error(t, err,
		"raw '$$' sqlite path whose interpreted form doesn't exist must be rejected at add time")
	require.Contains(t, err.Error(), "$$")

	// The escaped form works end-to-end (ping included).
	tr = testrun.New(th.Context, t, nil)
	escaped := "sqlite3:" + strings.ReplaceAll(fpath, "$", "$$")
	require.NoError(t, tr.Exec("add", escaped, "--handle", "@sl_esc_dollar", "--driver", "sqlite3"),
		"escaped sqlite form must pass the check and the add-time ping")
}

// TestCmdAdd_RawDollarDollar_DetectionErrorNamesTypedForm verifies
// that when neither the typed nor the interpreted path exists and
// driver detection fails, the error mentions the typed form and the
// '$$' interpretation, not just the interpreted path the user never
// typed.
func TestCmdAdd_RawDollarDollar_DetectionErrorNamesTypedForm(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Unrecognized extension and no file at either form: detection
	// byte-sniffs the interpreted path and fails.
	err := tr.Exec("add", filepath.Join(t.TempDir(), "data$$file.xyz"),
		"--handle", "@nofile_dollar", "--skip-verify")
	require.Error(t, err)
	require.Contains(t, err.Error(), "$$",
		"detection failure must surface the '$$' interpretation, not only the interpreted path")
}

// newRqliteMockHandlerForCLI returns an HTTP handler that satisfies gorqlite's
// cluster discovery protocol, suitable for CLI-level seam tests. hostPtr is a
// pointer to the "host:port" string of the test server resolved at request time
// so the handler can be constructed before the server URL is known.
//
// The handler responds to:
//   - GET /status  — returns a minimal JSON body with a top-level "node" key.
//   - GET /nodes   — returns the test server as the single leader, using the
//     scheme dictated by tlsNodes: "https" when true, "http" when false.
//   - GET /db/query — returns a minimal single-row result (used by PingContext).
//
// The tlsNodes parameter controls the scheme in the /nodes response. Set it to
// true when the test server itself is a TLS server (httptest.NewTLSServer), and
// false for plain HTTP servers. A mismatch causes gorqlite to connect to the
// wrong scheme during cluster discovery, and Ping will fail.
func newRqliteMockHandlerForCLI(hostPtr *string, tlsNodes bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/status":
			// Return a store.leader raft addr with no matching metadata api_addr
			// so gorqlite falls back to /nodes to resolve the HTTP API address.
			_, _ = w.Write([]byte(`{"node":{},"store":{"leader":"127.0.0.1:4002","metadata":{}}}`))
		case "/nodes":
			// Return the test server itself as the reachable leader with the
			// appropriate scheme so gorqlite connects via the right protocol.
			scheme := "http"
			if tlsNodes {
				scheme = "https"
			}
			body := fmt.Sprintf(
				`{"1":{"api_addr":"%s://%s","addr":"127.0.0.1:4002","reachable":true,"leader":true}}`,
				scheme, *hostPtr,
			)
			_, _ = w.Write([]byte(body))
		case "/db/query":
			_, _ = w.Write([]byte(`{"results":[{"columns":["1"],"types":["integer"],"values":[[1]]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// TestCmdAdd_Rqlite_ConnParamDetection exercises the sq add detection seam
// end-to-end against httptest servers. The TLS subtest swaps
// http.DefaultTransport so the detector's default-verification probe trusts the
// test server's cert; for that reason this test must not run in parallel.
func TestCmdAdd_Rqlite_ConnParamDetection(t *testing.T) {
	t.Run("tls_endpoint_detected_and_persisted", func(t *testing.T) {
		var host string
		server := httptest.NewTLSServer(newRqliteMockHandlerForCLI(&host, true))
		t.Cleanup(server.Close)
		host = server.Listener.Addr().String()

		// The CLI-built detector uses http.DefaultTransport (nil transport).
		// Swap in one that trusts the test cert so the HTTPS probe succeeds.
		ogTransport := http.DefaultTransport
		http.DefaultTransport = server.Client().Transport
		t.Cleanup(func() { http.DefaultTransport = ogTransport })

		tr := testrun.New(context.Background(), t, nil)
		err := tr.Exec("add", "--handle", "@rq_detect", "rqlite://"+host)
		require.NoError(t, err)

		src, err := tr.Run.Config.Collection.Get("@rq_detect")
		require.NoError(t, err)
		require.Contains(t, src.Location, "tls=true",
			"detection must persist tls=true for a TLS-only endpoint")
	})

	t.Run("plain_http_endpoint_not_rewritten", func(t *testing.T) {
		var host string
		server := httptest.NewServer(newRqliteMockHandlerForCLI(&host, false))
		t.Cleanup(server.Close)
		host = server.Listener.Addr().String()

		tr := testrun.New(context.Background(), t, nil)
		err := tr.Exec("add", "--handle", "@rq_plain", "rqlite://"+host)
		require.NoError(t, err)

		src, err := tr.Run.Config.Collection.Get("@rq_plain")
		require.NoError(t, err)
		require.NotContains(t, src.Location, "tls",
			"plain-HTTP endpoint must not be rewritten")
	})

	t.Run("skip_verify_means_zero_probes", func(t *testing.T) {
		var host string
		var hits atomic.Int64
		inner := newRqliteMockHandlerForCLI(&host, false)
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				inner.ServeHTTP(w, r)
			}))
		t.Cleanup(server.Close)
		host = server.Listener.Addr().String()

		tr := testrun.New(context.Background(), t, nil)
		err := tr.Exec("add", "--skip-verify", "--handle", "@rq_skip",
			"rqlite://"+host)
		require.NoError(t, err)
		require.Equal(t, int64(0), hits.Load(),
			"--skip-verify must suppress detection AND ping")
	})

	t.Run("placeholder_location_not_rewritten", func(t *testing.T) {
		// A ${env:...} placeholder location: detection must skip, so the
		// persisted location is byte-identical to the input.
		t.Setenv("SQ_TEST_RQ_DSN", "rqlite://localhost:4001")

		tr := testrun.New(context.Background(), t, nil)
		err := tr.Exec("add", "--skip-verify", "--driver", "rqlite",
			"--handle", "@rq_ph", "${env:SQ_TEST_RQ_DSN}")
		require.NoError(t, err)

		src, err := tr.Run.Config.Collection.Get("@rq_ph")
		require.NoError(t, err)
		require.Equal(t, "${env:SQ_TEST_RQ_DSN}", src.Location,
			"placeholder location must be persisted untouched")
	})

	t.Run("placeholder_gate_skips_detection", func(t *testing.T) {
		// Pins the placeholder gate in detectConnParamsForAdd
		// specifically (the prior subtest exits via the --skip-verify
		// gate and never reaches it). A TLS endpoint behind a
		// placeholder, WITHOUT --skip-verify and WITHOUT a transport
		// swap: the add must fail, and the failure mode reveals which
		// code ran. With the gate, detection is skipped and Ping fails
		// with the TLS-signal hint. Without the gate, detection would
		// run against the resolved location, hit the untrusted test
		// cert, and fail with the cert-verification error instead.
		var host string
		server := httptest.NewTLSServer(newRqliteMockHandlerForCLI(&host, true))
		t.Cleanup(server.Close)
		host = server.Listener.Addr().String()

		t.Setenv("SQ_TEST_RQ_TLS_DSN", "rqlite://"+host)

		tr := testrun.New(context.Background(), t, nil)
		err := tr.Exec("add", "--driver", "rqlite",
			"--handle", "@rq_ph_tls", "${env:SQ_TEST_RQ_TLS_DSN}")
		require.Error(t, err)
		require.Contains(t, err.Error(), "appears to require TLS",
			"failure must come from Ping enrichment, proving detection was skipped")
		require.NotContains(t, err.Error(), "certificate could not be verified",
			"detection's cert error would mean the placeholder gate did not fire")
	})
}

// TestFilterToAdvertisedParams pins the subset-invariant contract clause:
// detected keys not advertised by the driver's ConnParams are dropped (a driver
// bug must not fail the user's add).
func TestFilterToAdvertisedParams(t *testing.T) {
	tr := testrun.New(context.Background(), t, nil)
	drvr, err := tr.Run.DriverRegistry.DriverFor(drivertype.Rqlite)
	require.NoError(t, err)

	src := &source.Source{
		Handle:   "@rq",
		Type:     drivertype.Rqlite,
		Location: "rqlite://host:4001",
	}
	in := url.Values{
		"tls":           {"true"}, // advertised by rqlite ConnParams
		"bogusDetected": {"x"},    // NOT advertised: must be dropped
	}
	out := cli.FilterToAdvertisedParams(context.Background(), drvr, src, in)
	require.Equal(t, url.Values{"tls": []string{"true"}}, out)

	t.Run("non-sql driver drops all params", func(t *testing.T) {
		// A non-SQLDriver advertises no ConnParams, so nothing can be
		// validated against the subset invariant: everything is dropped.
		out := cli.FilterToAdvertisedParams(context.Background(), nonSQLDriverStub{}, src,
			url.Values{"tls": {"true"}})
		require.Empty(t, out)
	})
}

// nonSQLDriverStub implements driver.Driver but NOT driver.SQLDriver.
// Only the type assertion in filterToAdvertisedParams matters; the
// methods are never called.
type nonSQLDriverStub struct{}

func (nonSQLDriverStub) Open(context.Context, *source.Source) (driver.Grip, error) {
	panic("not implemented")
}

func (nonSQLDriverStub) Ping(context.Context, *source.Source) error {
	panic("not implemented")
}

func (nonSQLDriverStub) DriverMetadata() driver.Metadata {
	return driver.Metadata{}
}

func (nonSQLDriverStub) ValidateSource(*source.Source) (*source.Source, error) {
	panic("not implemented")
}

// dollarDirNames are directory names containing bytes significant in
// the placeholder-template grammar. Adding a source via a relative
// path from inside such a directory splices these filesystem bytes
// into the stored location template, which must escape them (gh #797).
var dollarDirNames = []string{
	"q$exports",
	"q$$exports",
	"${env:X}",
}

// chdirDollarDir creates dirName under a fresh temp dir, chdirs into
// it, and returns the resolved cwd (os.Getwd after chdir, so macOS
// /tmp symlinks don't skew expectations).
func chdirDollarDir(t *testing.T, dirName string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), dirName)
	require.NoError(t, os.Mkdir(dir, 0o750))
	t.Chdir(dir)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	return cwd
}

// TestCmdAdd_CwdDollarDir_CSV is the gh #797 round trip for the
// generic document-file path (location.Abs): sq add of a relative CSV
// path from inside a directory whose name contains '$' bytes must (a)
// succeed, (b) store a template that resolves back to the true
// filesystem path, (c) form no accidental ${scheme:path} ref, and (d)
// remain queryable end to end.
func TestCmdAdd_CwdDollarDir_CSV(t *testing.T) {
	for _, dirName := range dollarDirNames {
		t.Run(tu.Name(dirName), func(t *testing.T) {
			cwd := chdirDollarDir(t, dirName)
			require.NoError(t, os.WriteFile("actor.csv", []byte("a,b\n1,2\n3,4\n"), 0o600))

			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			const handle = "@actor797"
			// No --skip-verify: the post-add ping exercises the
			// connect-time resolution of the stored template.
			require.NoError(t, tr.Exec("add", "actor.csv", "--handle", handle),
				"sq add of a relative path must succeed when the cwd contains '$' bytes")

			src, err := tr.Run.Config.Collection.Get(handle)
			require.NoError(t, err)

			refs, err := secret.ExtractRefs(src.Location)
			require.NoError(t, err, "stored template must parse cleanly")
			require.Empty(t, refs, "filesystem-derived bytes must not form placeholder refs")

			resolvedSrc, err := driver.ResolveSourceSecrets(th.Context, src)
			require.NoError(t, err)
			require.Equal(t, filepath.Join(cwd, "actor.csv"), resolvedSrc.Location,
				"resolved location must be the true filesystem path")

			// And the source is actually queryable.
			tr = testrun.New(th.Context, t, tr)
			require.NoError(t, tr.Exec(".data", "--json"))
			var results []map[string]any
			tr.Bind(&results)
			require.Len(t, results, 2)
		})
	}
}

// TestCmdAdd_CwdDollarDir_FileDB is the gh #797 round trip for the
// file-DB munge path (sqlite3, duckdb). The sqlite3 case pings (which
// would create a stray DB file at a misinterpreted path if the cwd
// bytes weren't escaped); duckdb skips verification since no real
// duckdb database file is available here.
func TestCmdAdd_CwdDollarDir_FileDB(t *testing.T) {
	drvrs := []struct {
		typ        drivertype.Type
		prefix     string
		fname      string
		skipVerify bool
	}{
		{typ: drivertype.SQLite, prefix: "sqlite3://", fname: "sakila.db"},
		{typ: drivertype.DuckDB, prefix: "duckdb://", fname: "sakila.duckdb", skipVerify: true},
	}

	for _, drvr := range drvrs {
		for _, dirName := range dollarDirNames {
			t.Run(tu.Name(drvr.typ.String(), dirName), func(t *testing.T) {
				cwd := chdirDollarDir(t, dirName)
				require.NoError(t, os.WriteFile(drvr.fname, nil, 0o600))

				th := testh.New(t)
				tr := testrun.New(th.Context, t, nil)
				const handle = "@filedb797"
				args := []string{"add", drvr.fname, "--driver", drvr.typ.String(), "--handle", handle}
				if drvr.skipVerify {
					args = append(args, "--skip-verify")
				}
				require.NoError(t, tr.Exec(args...),
					"sq add of a relative path must succeed when the cwd contains '$' bytes")

				src, err := tr.Run.Config.Collection.Get(handle)
				require.NoError(t, err)

				wantTmpl := drvr.prefix + filepath.ToSlash(filepath.Join(secret.Escape(cwd), drvr.fname))
				require.Equal(t, wantTmpl, src.Location,
					"stored template must escape the cwd-derived bytes")

				refs, err := secret.ExtractRefs(src.Location)
				require.NoError(t, err, "stored template must parse cleanly")
				require.Empty(t, refs, "filesystem-derived bytes must not form placeholder refs")

				resolvedSrc, err := driver.ResolveSourceSecrets(th.Context, src)
				require.NoError(t, err)
				wantLit := drvr.prefix + filepath.ToSlash(filepath.Join(cwd, drvr.fname))
				require.Equal(t, wantLit, resolvedSrc.Location,
					"resolved location must be the true filesystem path")

				// The DB file the user pointed at must be the only file in
				// the dir: a misinterpreted path would have caused sqlite3
				// to create a stray DB elsewhere (or fail the ping).
				entries, err := os.ReadDir(cwd)
				require.NoError(t, err)
				require.Len(t, entries, 1)
				require.Equal(t, drvr.fname, entries[0].Name())
			})
		}
	}
}
