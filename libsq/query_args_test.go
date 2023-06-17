package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,lll
func TestQuery_args(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "arg_value_string",
			in:           `@sakila | .actor | select(.first_name == $first)`,
			args:         map[string]string{"first": "TOM"},
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecCount: 2,
		},
		{
			name:         "arg_value_string_2",
			in:           `@sakila | .actor | select(.first_name == $first && .last_name == $last)`,
			args:         map[string]string{"first": "TOM", "last": "MIRANDA"},
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'"},
			wantRecCount: 1,
		},
		{
			name:         "arg_value_int",
			in:           `@sakila | .actor | select(.actor_id == int($id))`,
			args:         map[string]string{"id": "1"},
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			skip:         true, // Skip until we implement casting, e.g. .actor_id == int($id)
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
