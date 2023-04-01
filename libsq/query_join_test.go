package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/sakila"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,lll
func TestQuery_join(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:     "join/single-selector",
			in:       `@sakila | .actor, .film_actor | join(.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecs: sakila.TblFilmActorCount,
		},
		{
			name:     "join/fq-table-cols-equal",
			in:       `@sakila | .actor, .film_actor | join(.film_actor.actor_id == .actor.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film_actor" ON "film_actor"."actor_id" = "actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film_actor` ON `film_actor`.`actor_id` = `actor`.`actor_id`"},
			wantRecs: sakila.TblFilmActorCount,
		},
		{
			name:     "join/fq-table-cols-equal-whitespace",
			in:       `@sakila | .actor, ."film actor" | join(."film actor".actor_id == .actor.actor_id)`,
			wantSQL:  `SELECT * FROM "actor" INNER JOIN "film actor" ON "film actor"."actor_id" = "actor"."actor_id"`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` INNER JOIN `film actor` ON `film actor`.`actor_id` = `actor`.`actor_id`"},
			skipExec: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
