package sqlite3_test

// TODO(gh527): Expand this file into a broader SQLite built-in function
// smoke suite (math, json, fts5, date/time, etc.) so future amalgamation
// drift surfaces quickly rather than as a user-reported missing-function bug.

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// minSQLiteMajor and minSQLiteMinor define the minimum embedded SQLite
// version required to satisfy GH #527: unistr() was added in 3.47.0, and
// if (as an inline alias of iif) is also available from that line forward.
const (
	minSQLiteMajor = 3
	minSQLiteMinor = 47
)

// TestSQLiteVersionFloor asserts that the SQLite version bundled via
// mattn/go-sqlite3 is at least 3.47.0. If this test fails, a dependency
// downgrade has re-introduced the missing-function regressions tracked
// in GH #527 (unistr, if/iif).
func TestSQLiteVersionFloor(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.SL3)
	sink, err := th.QuerySQL(src, nil, "SELECT sqlite_version()")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))

	got, ok := stringz.Val(sink.Recs[0][0]).(string)
	require.True(t, ok, "sqlite_version() should return a string")

	parts := strings.Split(got, ".")
	require.GreaterOrEqual(t, len(parts), 2, "unexpected sqlite_version format: %q", got)

	major, err := strconv.Atoi(parts[0])
	require.NoError(t, err, "parsing sqlite_version major: %q", got)
	minor, err := strconv.Atoi(parts[1])
	require.NoError(t, err, "parsing sqlite_version minor: %q", got)

	ok = major > minSQLiteMajor || (major == minSQLiteMajor && minor >= minSQLiteMinor)
	require.Truef(t, ok,
		"SQLite %s is below the %d.%d.0 floor; unistr() and if/iif() will regress (see GH #527)",
		got, minSQLiteMajor, minSQLiteMinor)
}

// TestSQLiteMissingFunctions_gh527 is a regression test for GH #527,
// which reported that unistr() and if() were unavailable in sq. Both are
// provided by SQLite 3.47.0+; this test asserts each function executes
// and returns the expected value.
func TestSQLiteMissingFunctions_gh527(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "unistr",
			query: `SELECT unistr('\2728')`,
			want:  "\u2728",
		},
		{
			name:  "iif",
			query: `SELECT iif(1=1, 'yes', 'no')`,
			want:  "yes",
		},
		{
			name:  "if",
			query: `SELECT if(1=1, 'yes', 'no')`,
			want:  "yes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(sakila.SL3)
			sink, err := th.QuerySQL(src, nil, tc.query)
			require.NoErrorf(t, err, "function %q failed; see GH #527", tc.name)
			require.Equal(t, 1, len(sink.Recs))

			got, ok := stringz.Val(sink.Recs[0][0]).(string)
			require.Truef(t, ok, "function %q returned non-string value: %T",
				tc.name, sink.Recs[0][0])
			require.Equalf(t, tc.want, got, "function %q", tc.name)
		})
	}
}
