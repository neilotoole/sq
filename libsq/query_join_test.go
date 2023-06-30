package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/jointype"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/neilotoole/sq/testh/sakila"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

var noPredicateJoinNames = []string{
	string(jointype.Cross),
	jointype.CrossAlias,
	string(jointype.Natural),
	jointype.NaturalAlias,
}

var innerJoins = []string{
	jointype.JoinAlias,
	string(jointype.Inner),
	jointype.InnerAlias,
}

var predicateJoinNames = lo.Without(jointype.AllValues(), noPredicateJoinNames...)

//nolint:exhaustive,lll
func TestQuery_join_single_source(t *testing.T) {
	// The test names below encode the following: JOIN_TYPE/number-of-joins/XYZ
	testCases := []queryTestCase{
		{
			name:          "join/n1/error/no-predicate",
			in:            `@sakila | .store | join(.address)`,
			wantErr:       true,
			repeatReplace: predicateJoinNames,
		},
		{
			name:          "join/n1/error/with-predicate",
			in:            `@sakila | .store | cross_join(.address, .address_id)`,
			wantErr:       true,
			repeatReplace: noPredicateJoinNames,
		},
		{
			name:          "join/n1/error/no-table",
			in:            `@sakila | .store | join()`,
			wantErr:       true,
			repeatReplace: jointype.AllValues(),
		},
		{
			name:          "join/n1/equals-no-alias",
			in:            `@sakila | .store | join(.address, .store.address_id == .address.address_id)`,
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "join/n1/equals-with-alias",
			in:            `@sakila | .store:s | join(.address:a, .s.address_id == .a.address_id)`,
			wantSQL:       `SELECT * FROM "store" AS "s" INNER JOIN "address" AS "a" ON "s"."address_id" = "a"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` AS `s` INNER JOIN `address` AS `a` ON `s`.`address_id` = `a`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "join/n2/equals-with-alias/unqualified-cols",
			in:            `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .first_name, .last_name, .title`,
			wantSQL:       `SELECT "first_name", "last_name", "title" FROM "actor" AS "a" INNER JOIN "film_actor" AS "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" AS "f" ON "fa"."film_id" = "f"."film_id"`,
			override:      driverMap{mysql.Type: "SELECT `first_name`, `last_name`, `title` FROM `actor` AS `a` INNER JOIN `film_actor` AS `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` AS `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
		},
		{
			name:          "join/n2/equals-with-alias/qualified-cols",
			in:            `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .a.first_name, .a.last_name, .f.title`,
			wantSQL:       `SELECT "a"."first_name", "a"."last_name", "f"."title" FROM "actor" AS "a" INNER JOIN "film_actor" AS "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" AS "f" ON "fa"."film_id" = "f"."film_id"`,
			override:      driverMap{mysql.Type: "SELECT `a`.`first_name`, `a`.`last_name`, `f`.`title` FROM `actor` AS `a` INNER JOIN `film_actor` AS `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` AS `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
		},
		{
			name:          "join/n1/single-selector-no-alias",
			in:            `@sakila | .store | join(.address, .address_id)`,
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "join/n1/single-selector-with-alias",
			in:            `@sakila | .store:s | join(.address:a, .address_id)`,
			wantSQL:       `SELECT * FROM "store" AS "s" INNER JOIN "address" AS "a" ON "s"."address_id" = "a"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` AS `s` INNER JOIN `address` AS `a` ON `s`.`address_id` = `a`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "cross-join/n1/no-constraint",
			in:            `@sakila | .store | cross_join(.address)`,
			wantSQL:       `SELECT * FROM "store" CROSS JOIN "address"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` CROSS JOIN `address`"},
			wantRecCount:  1206,
			repeatReplace: []string{string(jointype.Cross), jointype.CrossAlias},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive
func TestQuery_table_alias(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "table-alias",
			in:           `@sakila | .actor:a | .a.first_name`,
			wantSQL:      `SELECT "a"."first_name" FROM "actor" AS "a"`,
			override:     driverMap{mysql.Type: "SELECT `a`.`first_name` FROM `actor` AS `a`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "first_name"),
			},
		},
		{
			name:         "table-whitespace-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name`,
			wantSQL:      `SELECT "oy vey"."first_name" FROM "actor" AS "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` FROM `actor` AS `oy vey`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "first_name"),
			},
		},
		{
			name:         "table-whitespace-alias-with-col-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name:given_name`,
			wantSQL:      `SELECT "oy vey"."first_name" AS "given_name" FROM "actor" AS "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` AS `given_name` FROM `actor` AS `oy vey`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "given_name"),
			},
		},
		{
			name:         "table-whitespace-alias-with-col-whitespace-alias",
			in:           `@sakila | .actor:"oy vey" | ."oy vey".first_name:"oy vey"`,
			wantSQL:      `SELECT "oy vey"."first_name" AS "oy vey" FROM "actor" AS "oy vey"`,
			override:     driverMap{mysql.Type: "SELECT `oy vey`.`first_name` AS `oy vey` FROM `actor` AS `oy vey`"},
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
