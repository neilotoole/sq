package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

//nolint:exhaustive,lll
func TestQuery_where(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "operator/eq",
			in:      `@sakila | .actor | where(.actor_id == 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" = 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` = 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` = 100",
			},
			wantRecCount: 1,
		},
		{
			name:    "operator/ne",
			in:      `@sakila | .actor | where(.actor_id != 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" != 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` != 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` != 100",
			},
			wantRecCount: 199,
		},
		{
			name:    "operator/lt",
			in:      `@sakila | .actor | where(.actor_id < 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" < 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` < 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` < 100",
			},
			wantRecCount: 99,
		},
		{
			name:    "operator/lte",
			in:      `@sakila | .actor | where(.actor_id <= 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" <= 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` <= 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` <= 100",
			},
			wantRecCount: 100,
		},
		{
			name:    "operator/gt",
			in:      `@sakila | .actor | where(.actor_id > 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" > 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` > 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` > 100",
			},
			wantRecCount: 100,
		},
		{
			name:    "operator/gte",
			in:      `@sakila | .actor | where(.actor_id >= 100)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" >= 100`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` >= 100",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` >= 100",
			},
			wantRecCount: 101,
		},
		{
			name:    "error/no-args",
			in:      `@sakila | .actor | where()`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			wantErr: true,
		},
		{
			name:    "select_alias",
			in:      `@sakila | .actor | select(.actor_id == 1)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` = 1",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` = 1",
			},
			wantRecCount: 1,
		},
		{
			name:    "where_compound_1",
			in:      `@sakila | .actor | where(.actor_id >= 100 && .actor_id < 150)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" >= 100 AND "actor_id" < 150`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` >= 100 AND `actor_id` < 150",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` >= 100 AND `actor_id` < 150",
			},
			wantRecCount: 50,
		},
		{
			name:    "where_compound_2",
			in:      `@sakila | .actor | where(.actor_id >= 100 || (.actor_id < 150 && .first_name == "TOM"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" >= 100 OR ("actor_id" < 150 AND "first_name" = 'TOM')`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` >= 100 OR (`actor_id` < 150 AND `first_name` = 'TOM')",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` >= 100 OR (`actor_id` < 150 AND `first_name` = 'TOM')",
			},
			wantRecCount: 103,
		},
		{
			name:    "where_using_col_alias",
			in:      `@sakila | .actor | .first_name:given_name | where(.given_name == "TOM")`,
			wantSQL: `SELECT "first_name" AS "given_name" FROM "actor" WHERE "given_name" = 'TOM'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT `first_name` AS `given_name` FROM `actor` WHERE `given_name` = 'TOM'",
				drivertype.ClickHouse: "SELECT `first_name` AS `given_name` FROM `actor` WHERE `given_name` = 'TOM'",
			},
			wantRecCount: 2,
			// Skip because this only works on SQLite, not the other SQL databases.
			// I'm not sure if this will ever be implemented. Perhaps sq could look at
			// the ".given_name" element in the where clause, and substitute in the
			// unaliased value? Could be messy. Either which way, not a high priority.
			skip: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
