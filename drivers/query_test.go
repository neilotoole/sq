package drivers_test

import (
	"strings"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/libsq"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

// queryTestCase is used to test libsq's rendering of SLQ into SQL.
// It is probably the most important test struct in the codebase.
type queryTestCase struct {
	// name is the test name
	name string

	// skip indicates the test should be skipped. Useful for test cases
	// that we wantSQL to implement in the future.
	skip bool

	// in is the SLQ input. The "@sakila" handle is replaced
	// with the source's actual handle before an individual
	// test cases is executed.
	in string

	// args contains args for the "--args a b" mechanism. Typically empty.
	args map[string]string

	// wantErr indicates that an error is expected
	wantErr bool

	// wantSQL is the wanted SQL
	wantSQL string

	// override allows an alternative "wantSQL" for a specific driver type.
	// For example, MySQL uses backtick as the quote char, so it needs
	// a separate wantSQL string.
	override map[source.Type]string

	// onlyFor indicates that this test should only run on sources of
	// the specified types. When empty, the test is executed on all types.
	onlyFor []source.Type

	// skipExec indicates that the resulting query should not be executed.
	// Some SLQ inputs we wantSQL to test don't actually have corresponding
	// data in the Sakila datasets.
	skipExec bool

	// wantRecs is the number of expected records from actually executing
	// the query. This is N/A if skipExec is true.
	wantRecs int
}

func execQueryTestCase(t *testing.T, tc queryTestCase) {
	if tc.skip {
		t.Skip()
	}

	srcs := testh.New(t).NewSourceSet(sakila.SQLLatest()...)
	// srcs := testh.New(t).NewSourceSet(sakila.SL3) // FIXME: remove when done debugging
	for _, src := range srcs.Items() {
		src := src

		t.Run(string(src.Type), func(t *testing.T) {
			if len(tc.onlyFor) > 0 {
				if !slices.Contains(tc.onlyFor, src.Type) {
					t.Skip()
				}
			}

			in := strings.Replace(tc.in, "@sakila", src.Handle, 1)
			t.Logf(in)
			want := tc.wantSQL
			if overrideWant, ok := tc.override[src.Type]; ok {
				want = overrideWant
			}

			_, err := srcs.SetActive(src.Handle)
			require.NoError(t, err)

			th := testh.New(t)
			dbases := th.Databases()

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, th.Log, dbases, dbases, srcs, in)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, want, gotSQL)
			t.Log(gotSQL)

			if tc.skipExec {
				return
			}

			sink, err := th.QuerySLQ(in)
			require.NoError(t, err)
			require.Equal(t, tc.wantRecs, len(sink.Recs))
		})
	}
}

//nolint:exhaustive,lll
func TestSLQ2SQL(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:     "select/cols",
			in:       `@sakila | .actor | .first_name, .last_name`,
			wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
		{
			name:     "select/cols-whitespace-single-col",
			in:       `@sakila | .actor | ."first name"`,
			wantSQL:  `SELECT "first name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
			skipExec: true,
		},
		{
			name:     "select/cols-whitespace-multiple-cols",
			in:       `@sakila | .actor | .actor_id, ."first name", ."last name"`,
			wantSQL:  `SELECT "actor_id", "first name", "last name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `actor_id`, `first name`, `last name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
			skipExec: true,
		},
		{
			name:     "count/whitespace-col",
			in:       `@sakila | .actor | count(."first name")`,
			wantSQL:  `SELECT count("first name") FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT count(`first name`) FROM `actor`"},
			skipExec: true,
		},
		{
			name:     "select/table-whitespace",
			in:       `@sakila | ."film actor"`,
			wantSQL:  `SELECT * FROM "film actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `film actor`"},
			skipExec: true,
		},
		{
			name:     "select/cols-aliases",
			in:       `@sakila | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL:  `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name` AS `given_name`, `last_name` AS `family_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},

		{
			name:     "select/handle-table/cols",
			in:       `@sakila.actor | .first_name, .last_name`,
			wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
