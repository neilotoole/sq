package duckdb

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDriverMetadata verifies the static driver metadata. DuckDB is an
// embedded SQL driver.
func TestDriverMetadata(t *testing.T) {
	md := (&driveri{}).DriverMetadata()
	require.Equal(t, drivertype.DuckDB, md.Type)
	require.True(t, md.IsSQL)
	require.True(t, md.IsEmbeddedSQL)
	require.LessOrEqual(t, md.DefaultPort, 0)
}

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

			// MungeLocation must be idempotent: connect-time resolution
			// (driver.ResolveSourceSecrets) munges resolved placeholder
			// locations that may already be in canonical form (gh #798).
			again, err := MungeLocation(got)
			require.NoError(t, err)
			require.Equal(t, got, again, "MungeLocation must be idempotent")
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

// TestParseDuckDBGeneratedColumnNames_UnbalancedParenInLiteral verifies that
// a string literal containing an unbalanced closing paren does not confuse the
// outer column-list boundary scanner in parseDuckDBGeneratedColumnNames.
//
// The old scanner was not literal-aware: ':)' caused it to close the depth
// counter prematurely, truncating the column list so that the GENERATED ALWAYS
// AS column that follows was silently dropped (map entry absent, Generated not
// set).  After the fix the scanner tracks single-quoted literals and ignores
// parens inside them.
func TestParseDuckDBGeneratedColumnNames_UnbalancedParenInLiteral(t *testing.T) {
	ddl := `CREATE TABLE t (a INT, note VARCHAR DEFAULT ':)', doubled INT GENERATED ALWAYS AS (a*2))`
	got := parseDuckDBGeneratedColumnNames(ddl)
	require.NotNil(t, got, "result map must not be nil")
	require.True(t, got["doubled"],
		"'doubled' must be marked as generated; unbalanced paren in literal must not truncate column list")
	require.False(t, got["note"],
		"'note' must not be marked as generated (it has a DEFAULT, not GENERATED ALWAYS AS)")
	require.False(t, got["a"],
		"'a' must not be marked as generated (plain column)")
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
		// Column name containing a double-quote: DuckDB escapes it by
		// doubling, e.g. a"b → `'"a""b"'`. Must unquote to the real name,
		// not be misclassified as an expression sentinel.
		{in: `['"a""b"']`, want: []string{`a"b`}},
		{in: `['"a""b"', normal]`, want: []string{`a"b`, "normal"}},
		// Functional expression → recorded as a single sentinel.
		{in: `['(lower(email))']`, want: []string{""}},
		// Mixed simple + functional → sentinel preserves arity/position.
		{in: `[name, '(lower(email))']`, want: []string{"name", ""}},
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

// TestLogDroppedDuckDBIndex pins the warn-vs-debug discrimination
// added so that a future format change in duckdb_indexes().expressions
// surfaces as a warning, while legitimate empty / functional-only
// indexes log only at debug level.
func TestLogDroppedDuckDBIndex(t *testing.T) {
	testCases := []struct {
		name      string
		exprList  string
		wantLevel string // "DEBUG" or "WARN"
	}{
		{"canonical_empty", "[]", "DEBUG"},
		{"empty_with_spaces", "[ ]", "DEBUG"},
		{"all_functional", "['(lower(email))']", "DEBUG"},
		{"all_functional_multi", "['(lower(email))', '(upper(name))']", "DEBUG"},
		// Anything that doesn't look like a `[...]` list — a genuine
		// format change. These must warn so the issue is visible.
		{"unbracketed", "not-a-list", "WARN"},
		{"empty_string", "", "WARN"},
		{"unclosed_bracket", "[email", "WARN"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			logDroppedDuckDBIndex(log, "tbl", "idx", tc.exprList)

			line := strings.TrimSpace(buf.String())
			require.NotEmpty(t, line, "logDroppedDuckDBIndex must emit one entry per call")

			var entry map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &entry))
			require.Equal(t, tc.wantLevel, entry["level"],
				"input %q must log at %s level; got entry: %v", tc.exprList, tc.wantLevel, entry)
			require.Equal(t, tc.exprList, entry["expressions"])
		})
	}

	t.Run("nil_logger_is_safe", func(t *testing.T) {
		require.NotPanics(t, func() {
			logDroppedDuckDBIndex(nil, "tbl", "idx", "[]")
		})
	})
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

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "v1.5.2", want: "v1.5.2"}, // DuckDB version() is v-prefixed
		{raw: "1.1.3", want: "v1.1.3"},
		{raw: "not-a-version", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSemver(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
