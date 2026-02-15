package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive
func TestQuery_unique(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "unique/single-col",
			in:      `@sakila | .actor | .first_name | unique`,
			wantSQL: `SELECT DISTINCT "first_name" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT DISTINCT `first_name` FROM `actor`",
				drivertype.ClickHouse: "SELECT DISTINCT `first_name` FROM `actor`",
			},
			wantRecCount: 128,
		},
		{
			name:    "unique/no-col-with-parens",
			in:      `@sakila | .actor | unique()`,
			wantSQL: `SELECT DISTINCT * FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT DISTINCT * FROM `actor`",
				drivertype.ClickHouse: "SELECT DISTINCT * FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:    "unique/no-col",
			in:      `@sakila | .actor | unique`,
			wantSQL: `SELECT DISTINCT * FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT DISTINCT * FROM `actor`",
				drivertype.ClickHouse: "SELECT DISTINCT * FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:    "uniq/no-col",
			in:      `@sakila | .actor | uniq`,
			wantSQL: `SELECT DISTINCT * FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT DISTINCT * FROM `actor`",
				drivertype.ClickHouse: "SELECT DISTINCT * FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:    "uniq/no-col-with-parens",
			in:      `@sakila | .actor | uniq()`,
			wantSQL: `SELECT DISTINCT * FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT DISTINCT * FROM `actor`",
				drivertype.ClickHouse: "SELECT DISTINCT * FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
