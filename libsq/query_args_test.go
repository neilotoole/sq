package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive
func TestQuery_args(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:     "cols",
			in:       `@sakila | .actor | .$a`,
			args:     map[string]string{"a": "first_name"},
			wantSQL:  `SELECT "first_name", "last_name" FROM "actor"`,
			override: map[source.Type]string{mysql.Type: "SELECT `first_name`, `last_name` FROM `actor`"},
			wantRecs: sakila.TblActorCount,
			skip:     true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
