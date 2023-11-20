package libsq_test

import (
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/samber/lo"

	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tutil"
)

func TestQuery_join_args(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:          "error/missing-predicate",
			in:            `@sakila | .actor | join(.film_actor)`,
			wantErr:       true,
			repeatReplace: predicateJoinNames,
		},
		{
			name:          "error/unwanted-predicate",
			in:            `@sakila | .actor | join(.film_actor)`,
			wantErr:       true,
			repeatReplace: noPredicateJoinNames,
		},
		{
			name:          "error/too-many-args",
			in:            `@sakila | .actor | join(.film_actor, .actor_id, .first_name)`,
			wantErr:       true,
			repeatReplace: jointype.AllValues(),
		},
		{
			name:          "error/no-args",
			in:            `@sakila | .store | join()`,
			wantErr:       true,
			repeatReplace: jointype.AllValues(),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_join_inner(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "n1/equals-no-alias",
			in:           `@sakila | .store | join(.address, .store.address_id == .address.address_id)`,
			wantRecCount: 2,
		},
		{
			name:          "n1/equals-with-alias",
			in:            `@sakila | .store:s | join(.address:a, .s.address_id == .a.address_id)`,
			wantSQL:       `SELECT * FROM "store" AS "s" INNER JOIN "address" AS "a" ON "s"."address_id" = "a"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` AS `s` INNER JOIN `address` AS `a` ON `s`.`address_id` = `a`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "n2/equals-with-alias/unqualified-cols",
			in:            `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .first_name, .last_name, .title`,
			wantSQL:       `SELECT "first_name", "last_name", "title" FROM "actor" AS "a" INNER JOIN "film_actor" AS "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" AS "f" ON "fa"."film_id" = "f"."film_id"`,
			override:      driverMap{mysql.Type: "SELECT `first_name`, `last_name`, `title` FROM `actor` AS `a` INNER JOIN `film_actor` AS `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` AS `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
		},
		{
			name:          "n2/equals-with-alias/qualified-cols",
			in:            `@sakila | .actor:a | join(.film_actor:fa, .a.actor_id == .fa.actor_id) | join(.film:f, .fa.film_id == .f.film_id) | .a.first_name, .a.last_name, .f.title`,
			wantSQL:       `SELECT "a"."first_name", "a"."last_name", "f"."title" FROM "actor" AS "a" INNER JOIN "film_actor" AS "fa" ON "a"."actor_id" = "fa"."actor_id" INNER JOIN "film" AS "f" ON "fa"."film_id" = "f"."film_id"`,
			override:      driverMap{mysql.Type: "SELECT `a`.`first_name`, `a`.`last_name`, `f`.`title` FROM `actor` AS `a` INNER JOIN `film_actor` AS `fa` ON `a`.`actor_id` = `fa`.`actor_id` INNER JOIN `film` AS `f` ON `fa`.`film_id` = `f`.`film_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
		},
		{
			name:          "n1/single-selector-no-alias",
			in:            `@sakila | .store | join(.address, .address_id)`,
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "n1/table-handle-single-selector-no-alias",
			in:            `@sakila.store | join(.address, .address_id)`,
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` INNER JOIN `address` ON `store`.`address_id` = `address`.`address_id`"},
			wantRecCount:  2,
			repeatReplace: innerJoins,
		},
		{
			name:          "n1/single-selector-with-alias",
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

//nolint:lll
func TestQuery_join_multi_source(t *testing.T) {
	testCases := []queryTestCase{
		{
			name: "n1/equals-no-alias",
			in: fmt.Sprintf(
				`@sakila | .store | join(%s.address, .store.address_id == .address.address_id)`,
				sakila.SL3,
			),
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			wantRecCount:  2,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinStoreAddress...),
			},
		},
		{
			name: "n1/table-handle-equals-no-alias",
			in: fmt.Sprintf(
				`@sakila.store | join(%s.address, .store.address_id == .address.address_id)`,
				sakila.SL3,
			),
			wantSQL:       `SELECT * FROM "store" INNER JOIN "address" ON "store"."address_id" = "address"."address_id"`,
			wantRecCount:  2,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinStoreAddress...),
			},
		},
		{
			name: "n1/equals-with-alias",
			in: fmt.Sprintf(
				`@sakila | .store:s | join(%s.address:a, .s.address_id == .a.address_id)`,
				sakila.Pg,
			),
			wantRecCount:  2,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinStoreAddress...),
			},
		},
		{
			name: "n2/two-sources",
			in: fmt.Sprintf(
				`@sakila | .actor | join(%s.film_actor, .actor_id) | join(.film, .film_id) | .first_name, .last_name, .title`,
				sakila.Pg,
			),
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames("first_name", "last_name", "title"),
			},
		},
		{
			name: "n2/three-sources-no-alias-no-col-alias",
			in: fmt.Sprintf(
				`@sakila | .actor | join(%s.film_actor, .actor_id) | join(%s.film, .film_id) | .first_name, .last_name, .title`,
				sakila.Pg,
				sakila.My,
			),
			wantSQL:       `SELECT "first_name", "last_name", "title" FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id" INNER JOIN "film" ON "film_actor"."film_id" = "film"."film_id"`,
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames("first_name", "last_name", "title"),
			},
		},
		{
			name: "n2/three-sources-no-alias-all-cols",
			in: fmt.Sprintf(
				`@sakila | .actor | join(%s.film_actor, .actor_id) | join(%s.film, .film_id)`,
				sakila.Pg,
				sakila.My,
			),
			wantSQL:       `SELECT * FROM "actor" INNER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id" INNER JOIN "film" ON "film_actor"."film_id" = "film"."film_id"`,
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: innerJoins,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActorFilm...),
			},
		},
		{
			name: "n2/equals-with-alias/unqualified-cols",
			in: fmt.Sprintf(
				`@sakila | .actor:a | join(%s.film_actor:fa, .a.actor_id == .fa.actor_id) | join(%s.film:f, .fa.film_id == .f.film_id) | .first_name, .last_name, .title`,
				sakila.Pg,
				sakila.My,
			),
			wantRecCount: sakila.TblFilmActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames("first_name", "last_name", "title"),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

// TestQuery_join_others tests the join types other than INNER JOIN.
//
//nolint:exhaustive,lll
func TestQuery_join_others(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:          "left_join",
			in:            `@sakila | .actor | left_join(.film_actor, .actor_id)`,
			wantSQL:       `SELECT * FROM "actor" LEFT JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `actor` LEFT JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: []string{string(jointype.Left), jointype.LeftAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
		{
			name:          "left_outer_join",
			in:            `@sakila | .actor | left_outer_join(.film_actor, .actor_id)`,
			wantSQL:       `SELECT * FROM "actor" LEFT OUTER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `actor` LEFT OUTER JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: []string{string(jointype.LeftOuter), jointype.LeftOuterAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
		{
			name:          "right_join",
			in:            `@sakila | .actor | right_join(.film_actor, .actor_id)`,
			wantSQL:       `SELECT * FROM "actor" RIGHT JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `actor` RIGHT JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: []string{string(jointype.Right), jointype.RightAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
		{
			name:          "right_outer_join",
			in:            `@sakila | .actor | right_outer_join(.film_actor, .actor_id)`,
			wantSQL:       `SELECT * FROM "actor" RIGHT OUTER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `actor` RIGHT OUTER JOIN `film_actor` ON `actor`.`actor_id` = `film_actor`.`actor_id`"},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: []string{string(jointype.RightOuter), jointype.RightOuterAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
		{
			name:    "full_outer_join",
			in:      `@sakila | .actor | full_outer_join(.film_actor, .actor_id)`,
			wantSQL: `SELECT * FROM "actor" FULL OUTER JOIN "film_actor" ON "actor"."actor_id" = "film_actor"."actor_id"`,
			// Note that MySQL doesn't support full outer join.
			onlyFor:       []source.DriverType{sqlite3.Type, postgres.Type, sqlserver.Type},
			wantRecCount:  sakila.TblFilmActorCount,
			repeatReplace: []string{string(jointype.FullOuter), jointype.FullOuterAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
		{
			name: "full_outer_join/error-mysql",
			in:   `@sakila | .actor | full_outer_join(.film_actor, .actor_id)`,
			// Note that MySQL doesn't support full outer join.
			onlyFor:       []source.DriverType{mysql.Type},
			wantErr:       true,
			repeatReplace: []string{string(jointype.FullOuter), jointype.FullOuterAlias},
		},
		{
			name:          "cross/store-address",
			in:            `@sakila | .store | cross_join(.address)`,
			wantSQL:       `SELECT * FROM "store" CROSS JOIN "address"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` CROSS JOIN `address`"},
			wantRecCount:  1206,
			repeatReplace: []string{string(jointype.Cross), jointype.CrossAlias},
		},
		{
			name:          "cross/store-staff",
			in:            `@sakila | .store | cross_join(.staff)`,
			wantSQL:       `SELECT * FROM "store" CROSS JOIN "staff"`,
			override:      driverMap{mysql.Type: "SELECT * FROM `store` CROSS JOIN `staff`"},
			wantRecCount:  4,
			repeatReplace: []string{string(jointype.Cross), jointype.CrossAlias},
		},
		{
			name:          "cross/actor-film_actor/no-constraint",
			in:            `@sakila | .actor | cross_join(.film_actor) | .[0:10]`,
			wantRecCount:  10,
			repeatReplace: []string{string(jointype.Cross), jointype.CrossAlias},
			sinkFns: []SinkTestFunc{
				assertSinkColMungedNames(colsJoinActorFilmActor...),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

// TestQuery_table_alias is tested with the joins, because table aliases
// are primarily for use with join.
//
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

var (
	noPredicateJoinNames = []string{
		string(jointype.Cross),
		jointype.CrossAlias,
	}
	innerJoins = []string{
		jointype.JoinAlias,
		string(jointype.Inner),
	}
	predicateJoinNames     = lo.Without(jointype.AllValues(), noPredicateJoinNames...)
	colsJoinActorFilmActor = []string{
		"actor_id",
		"first_name",
		"last_name",
		"last_update",
		"actor_id_1",
		"film_id",
		"last_update_1",
	}
	colsJoinActorFilmActorFilm = []string{
		"actor_id",
		"first_name",
		"last_name",
		"last_update",
		"actor_id_1",
		"film_id",
		"last_update_1",
		"film_id_1",
		"title",
		"description",
		"release_year",
		"language_id",
		"original_language_id",
		"rental_duration",
		"rental_rate",
		"length",
		"replacement_cost",
		"rating",
		"special_features",
		"last_update_2",
	}
	colsJoinStoreAddress = []string{
		"store_id",
		"manager_staff_id",
		"address_id",
		"last_update",
		"address_id_1",
		"address",
		"address2",
		"district",
		"city_id",
		"postal_code",
		"phone",
		"last_update_1",
	}
)
