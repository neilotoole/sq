package sqlserver_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSLQ2SQL(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		handles []string
		slq     string
		wantSQL string
		wantErr bool
	}{
		{
			name:    "join",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL: `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
		},
		{
			name:    "select-cols",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor | .first_name, .last_name`,
			wantSQL: `SELECT "first_name", "last_name" FROM "actor"`,
		},
		{
			name:    "select-cols-aliases",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL: `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
		},
		{
			name:    "select-count-star",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor | count(*)`,
			wantSQL: `SELECT COUNT(*) FROM "actor"`,
		},
		{
			name:    "select-count",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor | count()`,
			wantSQL: `SELECT COUNT(*) FROM "actor"`,
		},
		{
			name:    "select-count-alias",
			handles: []string{sakila.MS17},
			slq:     `@sakila_ms17 | .actor | count(*):quantity`,
			wantSQL: `SELECT COUNT(*) AS "quantity" FROM "actor"`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
