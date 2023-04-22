package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,lll
func TestQuery_expr(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:     "literal/string",
			in:       `@sakila | .actor | .first_name == "TOM"`,
			wantSQL:  `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override: map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecs: 2,
		},
		{
			name:     "literal/two-strings",
			in:       `@sakila | .actor | .first_name == "TOM" && .last_name == "MIRANDA"`,
			wantSQL:  `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override: map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'"},
			wantRecs: 1,
		},
		{
			name:     "literal/integer",
			in:       `@sakila | .actor | .actor_id == 1`,
			wantSQL:  `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override: map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecs: 1,
		},
		{
			name:     "is_null",
			in:       `@sakila | .address | .postal_code == null`,
			wantSQL:  `SELECT * FROM "address" WHERE "postal_code" IS NULL`,
			override: map[source.DriverType]string{mysql.Type: "SELECT * FROM `address` WHERE `postal_code` IS NULL"},
			wantRecs: 4,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
		{
			name:     "is_not_null",
			in:       `@sakila | .address | .postal_code != null`,
			wantSQL:  `SELECT * FROM "address" WHERE "postal_code" IS NOT NULL`,
			override: map[source.DriverType]string{mysql.Type: "SELECT * FROM `address` WHERE `postal_code` IS NOT NULL"},
			wantRecs: 599,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
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
