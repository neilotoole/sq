package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_func(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "max",
			in:           `@sakila | .actor | max(.actor_id)`,
			wantSQL:      `SELECT max("actor_id") AS "max(.actor_id)" FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT max(`actor_id`) AS `max(.actor_id)` FROM `actor`"},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "max(.actor_id)"),
				assertSinkColValue(0, int64(200)),
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
