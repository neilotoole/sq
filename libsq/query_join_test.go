package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/neilotoole/sq/testh/sakila"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_table_alias(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "table-alias",
			in:           `@sakila | .actor:a | .a.first_name`,
			wantSQL:      `SELECT "a"."first_name" FROM "actor" "a"`,
			override:     driverMap{mysql.Type: "SELECT `a`.`first_name` FROM `actor` `a`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "first_name"),
			},
		},
		{
			name:         "table-whitespace-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name`,
			wantSQL:      `SELECT "oy vey"."first_name" FROM "actor" "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` FROM `actor` `oy vey`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "first_name"),
			},
		},
		{
			name:         "table-whitespace-alias-with-col-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name:given_name`,
			wantSQL:      `SELECT "oy vey"."first_name" AS "given_name" FROM "actor" "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` AS `given_name` FROM `actor` `oy vey`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "given_name"),
			},
		},
		{
			name:         "table-whitespace-alias-with-col-whitespace-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name:"oy vey"`,
			wantSQL:      `SELECT "oy vey"."first_name" AS "oy vey" FROM "actor" "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` AS `oy vey` FROM `actor` `oy vey`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "oy vey"),
			},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(i, tc.name), func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_join(t *testing.T) {
	testCases := []queryTestCase{
		{
			name: "join/n1/no-constraint",
			// We use .store and .address here, because using .actor and any
			// other table would result in a very large result.
			in:           `@sakila | .store | join(.address)`,
			wantSQL:      `SELECT * FROM "store" INNER JOIN "address"`,
			override:     driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address`"},
			wantRecCount: 1206,
		},
		{
			name:         "join/n1/equals-no-alias",
			in:           `@sakila | .store | join(.address, .store.address_id == .address.address_id)`,
			wantSQL:      `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			override:     driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
			wantRecCount: 2,
		},
		{
			name:         "join/n1/equals-with-alias",
			in:           `@sakila | .store:s | join(.address:a, .s.address_id == .a.address_id)`,
			wantSQL:      `SELECT * FROM "store" "s" INNER JOIN "address" "a" ON "s"."address_id" = "a"."address_id"`,
			override:     driverMap{mysql.Type: "SELECT * FROM `store` `s` INNER JOIN `address` `a` ON `s`.`address_id` = `a`.`address_id`"},
			wantRecCount: 2,
		},
		{
			name:         "join/n2/equals-with-alias/unqualified-cols",
			in:           `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .first_name, .last_name, .title`,
			wantSQL:      `SELECT "first_name", "last_name", "title" FROM "actor" "a" INNER JOIN "film_actor" "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" "f" ON "fa"."film_id" = "f"."film_id"`,
			override:     driverMap{mysql.Type: "SELECT `first_name`, `last_name`, `title` FROM `actor` `a` INNER JOIN `film_actor` `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount: sakila.TblFilmActorCount,
		},
		{
			name:         "join/n2/equals-with-alias/qualified-cols",
			in:           `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .a.first_name, .a.last_name, .f.title`,
			wantSQL:      `SELECT "a"."first_name", "a"."last_name", "f"."title" FROM "actor" "a" INNER JOIN "film_actor" "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" "f" ON "fa"."film_id" = "f"."film_id"`,
			override:     driverMap{mysql.Type: "SELECT `a`.`first_name`, `a`.`last_name`, `f`.`title` FROM `actor` `a` INNER JOIN `film_actor` `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount: sakila.TblFilmActorCount,
		},
		//{
		//	name:         "join/n1/single-selector-no-alias",
		//	in:           `@sakila | .store | join(.address, .address_id)`,
		//	wantSQL:      `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
		//	override:     driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
		//	wantRecCount: 2,
		//},
		//{
		//	name:         "join/n1/single-selector-with-alias",
		//	in:           `@sakila | .store:s | join(.address:a, .address_id)`,
		//	wantSQL:      `SELECT * FROM "store" "s" INNER JOIN "address" "a" ON "s"."address_id" = "a"."address_id"`,
		//	override:     driverMap{mysql.Type: "SELECT * FROM `store` `s` INNER JOIN `address` `a` ON `s`.`address_id` = `a`.`address_id`"},
		//	wantRecCount: 2,
		//},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
