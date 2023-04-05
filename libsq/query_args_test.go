package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,gocritic
func TestQuery_args(t *testing.T) {
	testCases := []queryTestCase{
		//{
		//	name:     "cols",
		//	in:       `@sakila | .actor | .$a`,
		//	args:     map[string]string{"a": "first_name"},
		//	wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
		//	override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
		//	wantRecs: sakila.TblActorCount,
		//	skip:     true,
		//},
		{
			name:     "cols",
			in:       `@sakila | .actor | .first_name == $name`,
			args:     map[string]string{"name": "TOM"},
			wantSQL:  `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecs: 2,
		},
		{
			name:     "cols2",
			in:       `@sakila | .actor | .first_name == "TOM"`,
			args:     map[string]string{"name": "TOM"},
			wantSQL:  `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override: map[source.Type]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecs: 2,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
