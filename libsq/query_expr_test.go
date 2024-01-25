package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/source/drivertype"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

//nolint:exhaustive,lll
func TestQuery_expr_where(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "literal/string",
			in:           `@sakila | .actor | where(.first_name == "TOM")`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override:     driverMap{drivertype.TypeMy: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'"},
			wantRecCount: 2,
		},
		{
			name:         "literal/two-strings",
			in:           `@sakila | .actor | where(.first_name == "TOM" && .last_name == "MIRANDA")`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override:     driverMap{drivertype.TypeMy: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'"},
			wantRecCount: 1,
		},
		{
			name:         "literal/integer",
			in:           `@sakila | .actor | where(.actor_id == 1)`,
			wantSQL:      `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override:     driverMap{drivertype.TypeMy: "SELECT * FROM `actor` WHERE `actor_id` = 1"},
			wantRecCount: 1,
		},
		{
			name:         "is_null",
			in:           `@sakila | .address | where(.postal_code == null)`,
			wantSQL:      `SELECT * FROM "address" WHERE "postal_code" IS NULL`,
			override:     driverMap{drivertype.TypeMy: "SELECT * FROM `address` WHERE `postal_code` IS NULL"},
			wantRecCount: 4,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
		{
			name:         "is_not_null",
			in:           `@sakila | .address | where(.postal_code != null)`,
			wantSQL:      `SELECT * FROM "address" WHERE "postal_code" IS NOT NULL`,
			override:     driverMap{drivertype.TypeMy: "SELECT * FROM `address` WHERE `postal_code` IS NOT NULL"},
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

//nolint:exhaustive
func TestQuery_expr_literal(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "table/col_and_literal",
			in:           `@sakila | .actor | .first_name, 1`,
			wantSQL:      `SELECT "first_name", 1 AS "1" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT `first_name`, 1 AS `1` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(1, int64(1))},
		},
		{
			name:         "table/literal",
			in:           `@sakila | .actor | 1`,
			wantSQL:      `SELECT 1 AS "1" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1 AS `1` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(1))},
		},
		{
			name:         "no-table/literal",
			in:           `@sakila | 1`,
			wantSQL:      `SELECT 1 AS "1"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1 AS `1`"},
			wantRecCount: 1,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(1))},
		},
		{
			name:         "no-table/literal_addition",
			in:           `@sakila | 1+1`,
			wantSQL:      `SELECT 1+1 AS "1+1"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1+1 AS `1+1`"},
			wantRecCount: 1,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(2))},
		},
		{
			name:         "table/literal_parens",
			in:           `@sakila | .actor | (1)`,
			wantSQL:      `SELECT (1) AS "(1)" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT (1) AS `(1)` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(1))},
		},
		{
			name:         "table/addition",
			in:           `@sakila | .actor | 1+2`,
			wantSQL:      `SELECT 1+2 AS "1+2" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1+2 AS `1+2` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(3))},
		},
		{
			name:         "table/addition_whitespace",
			in:           `@sakila | .actor | 1+ 2 +  3`,
			wantSQL:      `SELECT 1+2+3 AS "1+2+3" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1+2+3 AS `1+2+3` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(6))},
		},
		{
			name:         "table/math_parens",
			in:           `@sakila | .actor | ((2+2) *  3)`,
			wantSQL:      `SELECT ((2+2)*3) AS "((2+2)*3)" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT ((2+2)*3) AS `((2+2)*3)` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColValue(0, int64(12))},
		},
		{
			name:         "table/literal_alias",
			in:           `@sakila | .actor | 1:total`,
			wantSQL:      `SELECT 1 AS "total" FROM "actor"`,
			override:     driverMap{drivertype.TypeMy: "SELECT 1 AS `total` FROM `actor`"},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "total"),
				assertSinkColValue(0, int64(1)),
			},
		},
		{
			name:     "table/addition_alias",
			in:       `@sakila | .actor | (1+2):total`,
			wantSQL:  `SELECT (1+2) AS "total" FROM "actor"`,
			override: driverMap{drivertype.TypeMy: "SELECT (1+2) AS `total` FROM `actor`"},
			sinkFns: []SinkTestFunc{
				assertSinkColValue(0, int64(3)),
				assertSinkColName(0, "total"),
			},
			wantRecCount: sakila.TblActorCount,
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.name), func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
