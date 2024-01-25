package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive,lll
func TestQuery_cols(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "cols",
			in:           `@sakila | .actor | .first_name, .last_name`,
			wantSQL:      `SELECT "first_name", "last_name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "cols-whitespace-single-col",
			in:           `@sakila | .actor | ."first name"`,
			wantSQL:      `SELECT "first name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			skipExec:     true,
		},
		{
			name:         "cols-whitespace-multiple-cols",
			in:           `@sakila | .actor | .actor_id, ."first name", ."last name"`,
			wantSQL:      `SELECT "actor_id", "first name", "last name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `actor_id`, `first name`, `last name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			skipExec:     true,
		},
		{
			name:     "table-whitespace",
			in:       `@sakila | ."film actor"`,
			wantSQL:  `SELECT * FROM "film actor"`,
			override: driverMap{drivertype.MySQL: "SELECT * FROM `film actor`"},
			skipExec: true,
		},
		{
			name:         "cols-aliases",
			in:           `@sakila | .actor | .first_name:given_name, .last_name:family_name`,
			wantSQL:      `SELECT "first_name" AS "given_name", "last_name" AS "family_name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name` AS `given_name`, `last_name` AS `family_name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "cols-aliases-whitespace",
			in:           `@sakila | .actor | .first_name:"Given Name", .last_name:family_name`,
			wantSQL:      `SELECT "first_name" AS "Given Name", "last_name" AS "family_name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name` AS `Given Name`, `last_name` AS `family_name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "handle-table/cols",
			in:           `@sakila.actor | .first_name, .last_name`,
			wantSQL:      `SELECT "first_name", "last_name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "cols-select-literal-value",
			in:           `@sakila.actor | .first_name, "xxx", .last_name`,
			wantSQL:      `SELECT "first_name", 'xxx' AS "xxx", "last_name" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name`, 'xxx' AS `xxx`, `last_name` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "select/literal",
			in:           `@sakila.actor | .first_name, 5`,
			wantSQL:      `SELECT "first_name", 5 AS "5" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT `first_name`, 5 AS `5` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
