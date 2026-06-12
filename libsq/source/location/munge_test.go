package location_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/testh/tu"
)

func TestMungeForDriver_EmptyPath(t *testing.T) {
	testCases := []struct {
		name string
		typ  drivertype.Type
		loc  string
	}{
		{name: "sqlite3 prefix only", typ: drivertype.SQLite, loc: "sqlite3://"},
		{name: "sqlite3 bare scheme only", typ: drivertype.SQLite, loc: "sqlite3:"},
		{name: "sqlite3 empty path with query", typ: drivertype.SQLite, loc: "sqlite3://?mode=ro"},
		{name: "duckdb prefix only", typ: drivertype.DuckDB, loc: "duckdb://"},
		{name: "duckdb empty path with query", typ: drivertype.DuckDB, loc: "duckdb://?access_mode=READ_ONLY"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := location.MungeForDriver(tc.typ, tc.loc)
			require.Error(t, err)
			require.NotContains(t, err.Error(), tc.loc,
				"error must not echo the location")
		})
	}

	// The :memory: sentinel is not an empty path.
	got, err := location.MungeForDriver(drivertype.DuckDB, "duckdb://:memory:")
	require.NoError(t, err)
	require.Equal(t, "duckdb://:memory:", got)
}

// dollarDirNames are directory names containing bytes that are
// significant in the placeholder-template grammar. A cwd ending in one
// of these must be escaped when spliced into a stored location
// template (gh #797).
var dollarDirNames = []string{
	"q$exports",
	"q$$exports",
	"${env:X}",
}

// chdirNew creates dirName under a fresh temp dir, chdirs into it, and
// returns the resolved cwd (os.Getwd after chdir, so macOS /tmp
// symlinks don't skew expectations).
func chdirNew(t *testing.T, dirName string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), dirName)
	require.NoError(t, os.Mkdir(dir, 0o750))
	t.Chdir(dir)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	return cwd
}

// TestAbs_EscapesCwdDollarBytes verifies that location.Abs treats its
// result as a placeholder template: the cwd bytes it splices in are
// escaped (every filesystem '$' doubled), so unescaping the stored
// template yields the true filesystem path and no accidental
// ${scheme:path} ref is formed (gh #797).
func TestAbs_EscapesCwdDollarBytes(t *testing.T) {
	for _, dirName := range dollarDirNames {
		t.Run(tu.Name(dirName), func(t *testing.T) {
			cwd := chdirNew(t, dirName)

			got := location.Abs("report.csv")

			refs, err := secret.ExtractRefs(got)
			require.NoError(t, err, "stored template must parse cleanly")
			require.Empty(t, refs, "cwd bytes must not form placeholder refs")
			require.Equal(t, secret.Escape(cwd)+string(filepath.Separator)+"report.csv", got)
			require.Equal(t, filepath.Join(cwd, "report.csv"), secret.Unescape(got),
				"unescaped template must be the true filesystem path")
		})
	}
}

// TestAbs_PreservesUserTypedBytes verifies that only the cwd-derived
// bytes are escaped: the user's typed bytes (which are already
// template bytes, where '$$' means a literal '$') pass through
// exactly as typed.
func TestAbs_PreservesUserTypedBytes(t *testing.T) {
	t.Run("relative with typed escape", func(t *testing.T) {
		cwd := chdirNew(t, "q$$exports")
		got := location.Abs("data$$file.csv")
		require.Equal(t, secret.Escape(cwd)+string(filepath.Separator)+"data$$file.csv", got,
			"typed '$$' must not be re-escaped")
		require.Equal(t, filepath.Join(cwd, "data$file.csv"), secret.Unescape(got))
	})

	t.Run("absolute path untouched", func(t *testing.T) {
		// An absolute typed path contains no cwd-derived bytes: every
		// byte is the user's, so nothing is escaped.
		loc := filepath.Join(t.TempDir(), "q$$exports", "file.csv")
		require.Equal(t, loc, location.Abs(loc))
	})
}

// TestMungeTemplateForDriver_EscapesCwdDollarBytes verifies the same
// cwd-escape contract for the file-DB munge path (sqlite3, duckdb),
// and that the stored template round-trips at connect time: unescape
// plus literal-mode MungeForDriver yields the true filesystem path
// with no double-escape (gh #797).
func TestMungeTemplateForDriver_EscapesCwdDollarBytes(t *testing.T) {
	drvrs := []struct {
		typ    drivertype.Type
		prefix string
		fname  string
	}{
		{typ: drivertype.SQLite, prefix: "sqlite3://", fname: "sakila.db"},
		{typ: drivertype.DuckDB, prefix: "duckdb://", fname: "sakila.duckdb"},
	}

	for _, drvr := range drvrs {
		for _, dirName := range dollarDirNames {
			t.Run(tu.Name(drvr.typ.String(), dirName), func(t *testing.T) {
				cwd := chdirNew(t, dirName)

				got, err := location.MungeTemplateForDriver(drvr.typ, drvr.fname)
				require.NoError(t, err)

				wantTmpl := drvr.prefix + filepath.ToSlash(filepath.Join(secret.Escape(cwd), drvr.fname))
				require.Equal(t, wantTmpl, got)

				refs, err := secret.ExtractRefs(got)
				require.NoError(t, err, "stored template must parse cleanly")
				require.Empty(t, refs, "cwd bytes must not form placeholder refs")

				// Idempotent: re-munging the stored template must not
				// double-escape the already-escaped cwd bytes.
				again, err := location.MungeTemplateForDriver(drvr.typ, got)
				require.NoError(t, err)
				require.Equal(t, got, again)

				// Connect-time round trip (driver.ResolveSourceSecrets):
				// unescape the template, then literal-mode munge. The result
				// must be the true filesystem path, unescaped exactly once.
				resolved := secret.Unescape(got)
				litMunged, err := location.MungeForDriver(drvr.typ, resolved)
				require.NoError(t, err)
				wantLit := drvr.prefix + filepath.ToSlash(filepath.Join(cwd, drvr.fname))
				require.Equal(t, wantLit, litMunged)
			})
		}
	}
}

// TestMungeTemplateForDriver_RejectsPlaceholders verifies that the
// file-DB template munge enforces its ref-free contract: a location
// bearing a well-formed ${scheme:path} placeholder (or with malformed
// placeholder syntax) errors instead of being munged, and the error
// does not echo the location. Pass-through driver types stay
// pass-through, placeholders and all.
func TestMungeTemplateForDriver_RejectsPlaceholders(t *testing.T) {
	placeholderLocs := []string{
		"${env:SQ_DB_PATH}",
		"sqlite3://${env:SQ_DB_PATH}",
		"/var/db/${env:DB_NAME}.db",
		"sakila.db?key=${keyring:sakila/key}",
	}

	for _, typ := range []drivertype.Type{drivertype.SQLite, drivertype.DuckDB} {
		for _, loc := range placeholderLocs {
			t.Run(tu.Name(typ.String(), loc), func(t *testing.T) {
				_, err := location.MungeTemplateForDriver(typ, loc)
				require.Error(t, err)
				require.NotContains(t, err.Error(), loc,
					"error must not echo the location")
			})
		}

		t.Run(tu.Name(typ.String(), "malformed"), func(t *testing.T) {
			_, err := location.MungeTemplateForDriver(typ, "sakila${env:.db")
			require.Error(t, err)
			require.NotContains(t, err.Error(), "sakila",
				"error must not echo the location")
		})

		t.Run(tu.Name(typ.String(), "escaped dollar is ref-free"), func(t *testing.T) {
			// '$$' is the escape for a literal '$': no ref is formed, so
			// the template munges normally.
			got, err := location.MungeTemplateForDriver(typ, "/var/db/q$${env:X}.db")
			require.NoError(t, err)
			require.Equal(t, string(typ)+":///var/db/q$${env:X}.db", got)
		})
	}

	// A pass-through driver type is not inspected: the location returns
	// verbatim, placeholder or not.
	const pgLoc = "postgres://alice:${keyring:sakila/pw}@db/sakila"
	got, err := location.MungeTemplateForDriver(drivertype.Pg, pgLoc)
	require.NoError(t, err)
	require.Equal(t, pgLoc, got)
}

// TestMungeTemplateForDriver_QuerySuffixPreserved verifies that the
// "?key=val" connection-string suffix passes through template-mode
// munging verbatim, with only the cwd-derived path bytes escaped.
func TestMungeTemplateForDriver_QuerySuffixPreserved(t *testing.T) {
	cwd := chdirNew(t, "q$$exports")

	got, err := location.MungeTemplateForDriver(drivertype.SQLite, "sakila.db?mode=ro")
	require.NoError(t, err)
	want := "sqlite3://" + filepath.ToSlash(filepath.Join(secret.Escape(cwd), "sakila.db")) + "?mode=ro"
	require.Equal(t, want, got)
}
