package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_filter(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "filter/equal",
			in:           `@sakila | .actor | .actor_id == 1`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
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
