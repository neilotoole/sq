package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/sakila"
)

//nolint:exhaustive
func TestQuery_orderby(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "order_by/single-element",
			in:           `@sakila | .actor | order_by(.first_name)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name"`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "alias-ob",
			in:           `@sakila | .actor | ob(.first_name)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name"`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "order_by/single-element-table-selector",
			in:           `@sakila | .actor | order_by(.actor.first_name)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "actor"."first_name"`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `actor`.`first_name`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "order_by/single-element-asc",
			in:           `@sakila | .actor | order_by(.first_name+)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name" ASC`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name` ASC"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "order_by/single-element-desc",
			in:           `@sakila | .actor | order_by(.first_name-)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name" DESC`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name` DESC"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "order_by/multiple-elements",
			in:           `@sakila | .actor | order_by(.first_name+, .last_name-)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name" ASC, "last_name" DESC`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name` ASC, `last_name` DESC"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:         "order_by/synonym-sort-by",
			in:           `@sakila | .actor | sort_by(.first_name)`,
			wantSQL:      `SELECT * FROM "actor" ORDER BY "first_name"`,
			override:     driverMap{drivertype.MySQL: "SELECT * FROM `actor` ORDER BY `first_name`"},
			wantRecCount: sakila.TblActorCount,
		},
		{
			name:    "order_by/error-no-selector",
			in:      `@sakila | .actor | order_by()`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
