package sqlite3_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSLQ2SQL(t *testing.T) {
	// t.Parallel()

	testCases := []struct {
		name    string
		handles []string
		slq     string
		wantSQL string
		wantErr bool
	}{
		{
			name:    "select/cols",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor | .first_name, .last_name`,
			wantSQL: `SELECT "first_name", "last_name" FROM "actor"`,
		},
		{
			name:    "select/cols-whitespace",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace | .actor | ."first name"`,
			wantSQL: `SELECT "first name" FROM "actor"`,
		},
		{
			name:    "select/cols-whitespace-2",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace | .actor | .actor_id, ."first name", ."last name"`,
			wantSQL: `SELECT "actor_id", "first name", "last name" FROM "actor"`,
		},
		{
			name:    "select/count-whitespace-col",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace | .actor | count(."first name")`,
			wantSQL: `SELECT COUNT("first name") FROM "actor"`,
		},
		{
			name:    "select/table-whitespace",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace | ."film actor"`,
			wantSQL: `SELECT * FROM "film actor"`,
		},
		{
			name:    "select/cols-aliases",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL: `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
		},
		{
			name:    "select/count-star",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor | count(*)`,
			wantSQL: `SELECT COUNT(*) FROM "actor"`,
		},
		{
			name:    "select/count",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor | count()`,
			wantSQL: `SELECT COUNT(*) FROM "actor"`,
		},

		{
			name:    "select/handle-table/cols",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3.actor | .first_name, .last_name`,
			wantSQL: `SELECT "first_name", "last_name" FROM "actor"`,
		},
		{
			name:    "select/handle-table/count-star",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3.actor | count(*)`,
			wantSQL: `SELECT COUNT(*) FROM "actor"`,
		},
		{
			name:    "select/handle-table/count-col",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace.actor | count(."first name")`,
			wantSQL: `SELECT COUNT("first name") FROM "actor"`,
		},
		{
			name:    "select/count-alias",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor | count(*):quantity`,
			wantSQL: `SELECT COUNT(*) AS "quantity" FROM "actor"`,
		},
		{
			name:    "join/fq-table-cols-equal",
			handles: []string{sakila.SL3},
			slq:     `@sakila_sl3 | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL: `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
		},
		{
			name:    "join/fq-table-cols-equal-whitespace",
			handles: []string{sakila.SL3Whitespace},
			slq:     `@sakila_sl3_whitespace | .actor, ."film actor" | join(."film actor".actor_id == .actor.actor_id)`,
			wantSQL: `SELECT * FROM "actor" INNER JOIN "film actor" ON "film actor"."actor_id" = "actor"."actor_id"`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			// t.Parallel()
			t.Logf(tc.slq)

			th := testh.New(t)
			srcs := th.NewSourceSet(tc.handles...)
			_, err := srcs.SetActive(tc.handles[0])
			require.NoError(t, err)
			dbases := th.Databases()

			gotSQL, gotErr := libsq.SLQ2SQL(th.Context, th.Log, dbases, dbases, srcs, tc.slq)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)

			require.Equal(t, tc.wantSQL, gotSQL)
		})
	}
}
