package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

//nolint:exhaustive
func TestQuery_count(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "alias",
			in:           `@sakila | .actor | count:quantity`,
			wantSQL:      `SELECT count(*) AS "quantity" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `quantity` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:         "count-same-alias",
			in:           `@sakila | .actor | count:count`,
			wantSQL:      `SELECT count(*) AS "count" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:     "whitespace-col",
			in:       `@sakila | .actor | count(."first name")`,
			wantSQL:  `SELECT count("first name") FROM "actor"`,
			override: driverMap{drivertype.MySQL: "SELECT count(`first name`) FROM `actor`"},
			skipExec: true,
		},
		{
			name:         "select-handle-table",
			in:           `@sakila.actor | count`,
			wantSQL:      `SELECT count(*) AS "count" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:     "select-handle-table-ws-selector",
			in:       `@sakila.actor | count(."first name")`,
			wantSQL:  `SELECT count("first name") FROM "actor"`,
			override: driverMap{drivertype.MySQL: "SELECT count(`first name`) FROM `actor`"},
			skipExec: true,
		},
		{
			name: "no-parens-no-args-with-alias-unique",
			// Test that the count:ALIAS form can handle the alias
			// being a reserved word (unique).
			in:           `@sakila | .actor | count:unique`,
			wantSQL:      `SELECT count(*) AS "unique" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `unique` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:         "no-parens-no-args-with-alias-arbitrary",
			in:           `@sakila | .actor | count:something_123`,
			wantSQL:      `SELECT count(*) AS "something_123" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `something_123` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:         "parens-no-args",
			in:           `@sakila | .actor | count()`,
			wantSQL:      `SELECT count(*) AS "count" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:    "error-star",
			in:      `@sakila | .actor | count(*)`,
			wantErr: true, // Star version is not supported
		},
		{
			name:         "single-selector",
			in:           `@sakila | .actor | count(.first_name)`,
			wantSQL:      `SELECT count("first_name") FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(`first_name`) FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name:    "error-multiple-selector",
			in:      `@sakila | .actor | count(.first_name, .last_name)`,
			wantErr: true, // Only a single selector is permitted
		},
		{
			name:         "count/no-parens-no-args",
			in:           `@sakila | .actor | count`,
			wantSQL:      `SELECT count(*) AS "count" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecCount: 1,
		},
		{
			name: "count/no-parens-no-args-with-alias-count",
			// Test that the count:ALIAS form can handle the alias
			// being a reserved word (count).
			in:           `@sakila | .actor | count:count`,
			wantSQL:      `SELECT count(*) AS "count" FROM "actor"`,
			override:     driverMap{drivertype.MySQL: "SELECT count(*) AS `count` FROM `actor`"},
			wantRecCount: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
