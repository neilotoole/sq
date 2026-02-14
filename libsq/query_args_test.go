package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

//nolint:exhaustive,lll
func TestQuery_args(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "arg_value_string",
			in:           `@sakila | .actor | where(.first_name == $first)`,
			args:         map[string]string{"first": "TOM"},
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecCount: 2,
		},
		{
			name:         "arg_value_string_2",
			in:           `@sakila | .actor | where(.first_name == $first && .last_name == $last)`,
			args:         map[string]string{"first": "TOM", "last": "MIRANDA"},
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'"},
			wantRecCount: 1,
		},
		{
			name:         "arg_value_int",
			in:           `@sakila | .actor | where(.actor_id == int($id))`,
			args:         map[string]string{"id": "1"},
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			skip:         true, // Skip until we implement casting, e.g. .actor_id == int($id)
			wantRecCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
