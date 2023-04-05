package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_expr(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:     "literal/string",
			in:       `@sakila | .actor | .first_name == "TOM"`,
			wantSQL:  `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecs: 2,
		},
		{
			name:     "literal/integer",
			in:       `@sakila | .actor | .actor_id == 1`,
			wantSQL:  `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecs: 1,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
