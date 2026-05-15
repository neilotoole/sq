package duckdb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func TestMungeLocation(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	cwd = filepath.ToSlash(cwd)

	root, err := filepath.Abs("/")
	require.NoError(t, err)
	root = filepath.ToSlash(root)

	cwdWant := Prefix + cwd + "/foo.duckdb"

	t.Log("cwdWant:", cwdWant)
	t.Log("root:", root)

	testCases := []struct {
		in        string
		want      string
		onlyForOS string
		wantErr   bool
	}{
		{
			in:      "",
			wantErr: true,
		},
		{
			in:   ":memory:",
			want: Prefix + ":memory:",
		},
		{
			in:   Prefix + ":memory:",
			want: Prefix + ":memory:",
		},
		{
			in:   "duckdb:///path/to/foo.duckdb",
			want: Prefix + root + "path/to/foo.duckdb",
		},
		{
			in:   "duckdb://foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "duckdb:foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "duckdb:/foo.duckdb",
			want: Prefix + root + "foo.duckdb",
		},
		{
			in:   "foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "./foo.duckdb",
			want: cwdWant,
		},
		{
			in:   "/path/to/foo.duckdb",
			want: Prefix + root + "path/to/foo.duckdb",
		},
		{
			in:   `C:/Users/neil/work/sq/drivers/duckdb/testdata/sakila.duckdb`,
			want: `duckdb://C:/Users/neil/work/sq/drivers/duckdb/testdata/sakila.duckdb`,
			// The current impl of MungeLocation relies upon OS-specific functions
			// in pkg filepath. Thus, we skip this test on non-Windows OSes.
			// MungeLocation could probably be rewritten to be OS-independent?
			onlyForOS: "windows",
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			if tc.onlyForOS != "" && tc.onlyForOS != runtime.GOOS {
				t.Skipf("Skipping because this test is only for OS {%s}, but have {%s}",
					tc.onlyForOS, runtime.GOOS)
				return
			}

			got, err := MungeLocation(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDsnFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "", wantErr: true},
		{loc: "sqlite3://x", wantErr: true},
		{loc: "duckdb://", want: ""},
		{loc: Prefix + ":memory:", want: ":memory:"},
		{loc: Prefix + ":memory:?threads=4", want: ":memory:?threads=4"},
		{loc: Prefix + "/path/to/foo.duckdb", want: "/path/to/foo.duckdb"},
		{loc: Prefix + "/path/to/foo.duckdb?access_mode=READ_ONLY", want: "/path/to/foo.duckdb?access_mode=READ_ONLY"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, err := dsnFromLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestFilePathFromLocation(t *testing.T) {
	testCases := []struct {
		loc  string
		want string
	}{
		{loc: "", want: ""},
		{loc: "sqlite3:///foo.db", want: ""},
		{loc: Prefix, want: ""},
		{loc: Prefix + ":memory:", want: ""},
		{loc: Prefix + ":memory:?threads=4", want: ""},
		{loc: Prefix + "/path/to/foo.duckdb", want: "/path/to/foo.duckdb"},
		{loc: Prefix + "/path/to/foo.duckdb?access_mode=READ_ONLY", want: "/path/to/foo.duckdb"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			require.Equal(t, tc.want, filePathFromLocation(tc.loc))
		})
	}
}

func TestPathFromLocation(t *testing.T) {
	testCases := []struct {
		srcType drivertype.Type
		loc     string
		want    string
		wantErr bool
	}{
		{srcType: drivertype.SQLite, loc: Prefix + "/foo.duckdb", wantErr: true},
		{srcType: drivertype.DuckDB, loc: Prefix + ":memory:", wantErr: true},
		{srcType: drivertype.DuckDB, loc: Prefix + ":memory:?threads=4", wantErr: true},
		{srcType: drivertype.DuckDB, loc: Prefix + "/path/to/foo.duckdb", want: "/path/to/foo.duckdb"},
		{srcType: drivertype.DuckDB, loc: Prefix + "/path/to/foo.duckdb?access_mode=READ_ONLY", want: "/path/to/foo.duckdb"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.srcType, tc.loc), func(t *testing.T) {
			src := &source.Source{
				Handle:   "@h",
				Type:     tc.srcType,
				Location: tc.loc,
			}
			got, err := PathFromLocation(src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestParseDuckDBIndexExpressions covers the shapes that
// duckdb_indexes().expressions emits without needing a live DB.
func TestParseDuckDBIndexExpressions(t *testing.T) {
	testCases := []struct {
		in   string
		want []string
	}{
		{in: "[email]", want: []string{"email"}},
		{in: "[store_id, film_id]", want: []string{"store_id", "film_id"}},
		// Reserved-word column → DuckDB re-quotes as `'"name"'`.
		{in: `['"name"']`, want: []string{"name"}},
		// Mixed: bare + re-quoted.
		{in: `['"name"', email]`, want: []string{"name", "email"}},
		// Functional expression → no plain column ref → empty result.
		{in: `['(lower(email))']`, want: []string{}},
		// Mixed simple + functional → only the simple key survives.
		{in: `[name, '(lower(email))']`, want: []string{"name"}},
		// Pathological / unparseable inputs return nil.
		{in: "", want: nil},
		{in: "[]", want: nil},
		{in: "not-a-list", want: nil},
	}
	for _, tc := range testCases {
		t.Run(tu.Name(tc.in), func(t *testing.T) {
			got := parseDuckDBIndexExpressions(tc.in)
			switch {
			case tc.want == nil:
				require.Nil(t, got,
					"unparseable input must return a nil slice, not an empty one")
			case len(tc.want) == 0:
				require.NotNil(t, got,
					"a parsed list with no plain column refs must return an empty (non-nil) slice")
				require.Empty(t, got)
			default:
				require.Equal(t, tc.want, got)
			}
		})
	}
}

// TestConnParams asserts the whitelist's keys and known-value enumerations,
// so a typo (e.g. "READ ONLY") that would silently degrade tab completion
// is caught.
func TestConnParams(t *testing.T) {
	d := &driveri{}
	params := d.ConnParams()

	wantKeys := []string{
		"access_mode", "memory_limit", "threads", "default_order",
		"default_null_order", "enable_external_access", "enable_object_cache",
		"temp_directory", "wal_autocheckpoint",
	}
	for _, k := range wantKeys {
		_, ok := params[k]
		require.True(t, ok, "ConnParams missing expected key: %s", k)
	}

	require.ElementsMatch(t, []string{"READ_ONLY", "READ_WRITE"}, params["access_mode"])
	require.ElementsMatch(t, []string{"ASC", "DESC"}, params["default_order"])
	require.ElementsMatch(t, []string{"NULLS_FIRST", "NULLS_LAST"}, params["default_null_order"])
	require.ElementsMatch(t, []string{"true", "false"}, params["enable_external_access"])
	require.ElementsMatch(t, []string{"true", "false"}, params["enable_object_cache"])
}
