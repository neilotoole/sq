package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_where(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "col_val_equal_literal",
			in:           `@sakila | .actor | where(.actor_id == 1)`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecCount: 1,
		},
		{
			name:    "error/no-args",
			in:      `@sakila | .actor | where()`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			wantErr: true,
		},
		{
			name:         "alias_select",
			in:           `@sakila | .actor | select(.actor_id == 1)`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecCount: 1,
		},
		{
			name:         "where_compound_1",
			in:           `@sakila | .actor | where(.actor_id >= 100 && .actor_id < 150)`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" >= 100 AND "actor_id" < 150`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` >= 100 AND `actor_id` < 150"},
			wantRecCount: 50,
		},
		{
			name:         "where_compound_2",
			in:           `@sakila | .actor | where(.actor_id >= 100 || (.actor_id < 150 && .first_name == "TOM"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" >= 100 OR ("actor_id" < 150 AND "first_name" = 'TOM')`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` >= 100 OR (`actor_id` < 150 AND `first_name` = 'TOM')"},
			wantRecCount: 103,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
