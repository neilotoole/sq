package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/sakila"

	"github.com/neilotoole/sq/drivers/mysql"

	"github.com/neilotoole/sq/libsq/source"

	_ "github.com/mattn/go-sqlite3"
)

//nolint:exhaustive,lll
func TestQuery_expr(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "literal/string",
			in:           `@sakila | .actor | .first_name == "TOM"`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecCount: 2,
		},
		{
			name:         "literal/two-strings",
			in:           `@sakila | .actor | .first_name == "TOM" && .last_name == "MIRANDA"`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'"},
			wantRecCount: 1,
		},
		{
			name:         "literal/integer",
			in:           `@sakila | .actor | .actor_id == 1`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecCount: 1,
		},
		{
			name:         "is_null",
			in:           `@sakila | .address | .postal_code == null`,
			wantSQL:      `SELECT * FROM "address" WHERE "postal_code" IS NULL`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `address` WHERE `postal_code` IS NULL"},
			wantRecCount: 4,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
		{
			name:         "is_not_null",
			in:           `@sakila | .address | .postal_code != null`,
			wantSQL:      `SELECT * FROM "address" WHERE "postal_code" IS NOT NULL`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT * FROM `address` WHERE `postal_code` IS NOT NULL"},
			wantRecCount: 599,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_expr_simple(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "col_and_literal",
			in:           `@sakila | .actor | .first_name, 1`,
			wantSQL:      `SELECT "first_name", 1 FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT `first_name`, 1 FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(1, int64(1))},
		},
		{
			name:         "literal",
			in:           `@sakila | .actor | 1`,
			wantSQL:      `SELECT 1 FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT 1 FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(1))},
		},
		{
			name:         "literal_parens",
			in:           `@sakila | .actor | (1)`,
			wantSQL:      `SELECT (1) FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT (1) FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(1))},
		},
		{
			name:         "addition",
			in:           `@sakila | .actor | 1+2`,
			wantSQL:      `SELECT 1 + 2 FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT 1 + 2 FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(3))},
		},
		{
			name:         "addition_whitespace",
			in:           `@sakila | .actor | 1+ 2 +  3`,
			wantSQL:      `SELECT 1 + 2 + 3 FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT 1 + 2 + 3 FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(6))},
		},
		{
			name:         "math_parens",
			in:           `@sakila | .actor | (1+ 2) *  3`,
			wantSQL:      `SELECT (1 + 2) * 3 FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT (1 + 2) * 3 FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(9))},
		},
		{
			name:         "literal_alias",
			in:           `@sakila | .actor | 1:total`,
			wantSQL:      `SELECT 1 AS "total" FROM "actor"`,
			override:     map[source.DriverType]string{mysql.Type: "SELECT 1 AS `total` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "total"),
				assertSinkColValue(0, int64(1)),
			},
		},
		{
			name:     "addition_alias",
			in:       `@sakila | .actor | (1+2):total`,
			wantSQL:  `SELECT (1 + 2) AS "total" FROM "actor"`,
			override: map[source.DriverType]string{mysql.Type: "SELECT (1 + 2) AS `total` FROM `actor`"},
			sinkFns: []SinkTestFunc{
				assertSinkColValue(0, int64(3)),
				assertSinkColName(0, "total"),
			},
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
