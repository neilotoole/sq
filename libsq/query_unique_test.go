package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/sakila"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive
func TestQuery_unique(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "unique/single-col",
			in:           `@sakila | .actor | .first_name | unique`,
			wantSQL:      `SELECT DISTINCT "first_name" FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT DISTINCT `first_name` FROM `actor`"},
			wantRecCount: 128,
		},
		{
			name:         "unique/no-col",
			in:           `@sakila | .actor | unique`,
			wantSQL:      `SELECT DISTINCT * FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT DISTINCT * FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "unique/no-col",
			in:           `@sakila | .actor | unique`,
			wantSQL:      `SELECT DISTINCT * FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT DISTINCT * FROM `actor`"},
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
