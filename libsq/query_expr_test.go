package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

//nolint:exhaustive
func TestQuery_expr_where(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "literal/string",
			in:      `@sakila | .actor | where(.first_name == "TOM")`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" = 'TOM'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` = 'TOM'",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` = 'TOM'",
			},
			wantRecCount: 2,
		},
		{
			name:    "literal/two-strings",
			in:      `@sakila | .actor | where(.first_name == "TOM" && .last_name == "MIRANDA")`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" = 'TOM' AND "last_name" = 'MIRANDA'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` = 'TOM' AND `last_name` = 'MIRANDA'",
			},
			wantRecCount: 1,
		},
		{
			name:    "literal/integer",
			in:      `@sakila | .actor | where(.actor_id == 1)`,
			wantSQL: `SELECT * FROM "actor" WHERE "actor_id" = 1`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `actor_id` = 1",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `actor_id` = 1",
			},
			wantRecCount: 1,
		},
		{
			name:    "is_null",
			in:      `@sakila | .address | where(.postal_code == null)`,
			wantSQL: `SELECT * FROM "address" WHERE "postal_code" IS NULL`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `address` WHERE `postal_code` IS NULL",
				drivertype.ClickHouse: "SELECT * FROM `address` WHERE `postal_code` IS NULL",
			},
			wantRecCount: 4,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
		{
			name:    "is_not_null",
			in:      `@sakila | .address | where(.postal_code != null)`,
			wantSQL: `SELECT * FROM "address" WHERE "postal_code" IS NOT NULL`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `address` WHERE `postal_code` IS NOT NULL",
				drivertype.ClickHouse: "SELECT * FROM `address` WHERE `postal_code` IS NOT NULL",
			},
			wantRecCount: 599,
			// skipExec because mysql sakila db doesn't have the same null values.
			// This is a bug in the dataset.
			skipExec: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive
func TestQuery_expr_literal(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "table/col_and_literal",
			in:      `@sakila | .actor | .first_name, 1`,
			wantSQL: `SELECT "first_name", 1 AS "1" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT `first_name`, 1 AS `1` FROM `actor`",
				drivertype.ClickHouse: "SELECT `first_name`, 1 AS `1` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(1, 1)},
		},
		{
			name:    "table/literal",
			in:      `@sakila | .actor | 1`,
			wantSQL: `SELECT 1 AS "1" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1 AS `1` FROM `actor`",
				drivertype.ClickHouse: "SELECT 1 AS `1` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 1)},
		},
		{
			name:    "no-table/literal",
			in:      `@sakila | 1`,
			wantSQL: `SELECT 1 AS "1"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1 AS `1`",
				drivertype.ClickHouse: "SELECT 1 AS `1`",
				drivertype.Oracle:     `SELECT 1 AS "1" FROM DUAL`,
			},
			wantRecCount: 1,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 1)},
		},
		{
			name:    "no-table/literal_addition",
			in:      `@sakila | 1+1`,
			wantSQL: `SELECT 1+1 AS "1+1"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1+1 AS `1+1`",
				drivertype.ClickHouse: "SELECT 1+1 AS `1+1`",
				drivertype.Oracle:     `SELECT 1+1 AS "1+1" FROM DUAL`,
			},
			wantRecCount: 1,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 2)},
		},
		{
			name:    "table/literal_parens",
			in:      `@sakila | .actor | (1)`,
			wantSQL: `SELECT (1) AS "(1)" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT (1) AS `(1)` FROM `actor`",
				drivertype.ClickHouse: "SELECT (1) AS `(1)` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 1)},
		},
		{
			name:    "table/addition",
			in:      `@sakila | .actor | 1+2`,
			wantSQL: `SELECT 1+2 AS "1+2" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1+2 AS `1+2` FROM `actor`",
				drivertype.ClickHouse: "SELECT 1+2 AS `1+2` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 3)},
		},
		{
			name:    "table/addition_whitespace",
			in:      `@sakila | .actor | 1+ 2 +  3`,
			wantSQL: `SELECT 1+2+3 AS "1+2+3" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1+2+3 AS `1+2+3` FROM `actor`",
				drivertype.ClickHouse: "SELECT 1+2+3 AS `1+2+3` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 6)},
		},
		{
			name:    "table/math_parens",
			in:      `@sakila | .actor | ((2+2) *  3)`,
			wantSQL: `SELECT ((2+2)*3) AS "((2+2)*3)" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT ((2+2)*3) AS `((2+2)*3)` FROM `actor`",
				drivertype.ClickHouse: "SELECT ((2+2)*3) AS `((2+2)*3)` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns:      []SinkTestFunc{assertSinkColInt(0, 12)},
		},
		{
			name:    "table/literal_alias",
			in:      `@sakila | .actor | 1:total`,
			wantSQL: `SELECT 1 AS "total" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT 1 AS `total` FROM `actor`",
				drivertype.ClickHouse: "SELECT 1 AS `total` FROM `actor`",
			},
			wantRecCount: sakila.TblActorCount,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "total"),
				assertSinkColInt(0, 1),
			},
		},
		{
			name:    "table/addition_alias",
			in:      `@sakila | .actor | (1+2):total`,
			wantSQL: `SELECT (1+2) AS "total" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT (1+2) AS `total` FROM `actor`",
				drivertype.ClickHouse: "SELECT (1+2) AS `total` FROM `actor`",
			},
			sinkFns: []SinkTestFunc{
				assertSinkColInt(0, 3),
				assertSinkColName(0, "total"),
			},
			wantRecCount: sakila.TblActorCount,
		},
		{
			// A reserved word as an expression alias is applied, and the
			// expression is no longer mangled into the alias. See issue #646.
			name:    "table/addition_alias_reserved_word",
			in:      `@sakila | .actor | (1+2):count`,
			wantSQL: `SELECT (1+2) AS "count" FROM "actor"`,
			override: driverMap{
				drivertype.MySQL:      "SELECT (1+2) AS `count` FROM `actor`",
				drivertype.ClickHouse: "SELECT (1+2) AS `count` FROM `actor`",
			},
			sinkFns: []SinkTestFunc{
				assertSinkColInt(0, 3),
				assertSinkColName(0, "count"),
			},
			wantRecCount: sakila.TblActorCount,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.name), func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

// TestQuery_oracle_fractionalNumber guards #844 end-to-end: on Oracle a computed
// NUMBER with a fractional value (here, division) must surface as a decimal rather
// than crash an int64 scan. Oracle-only because division semantics and the result
// type differ across drivers (#838), so a cross-driver value assertion is not
// meaningful; the integer-division drivers would truncate to 7, not 7.25.
func TestQuery_oracle_fractionalNumber(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "division",
			in:           `@sakila | .actor | where(.actor_id == 58) | (.actor_id / 8)`,
			onlyFor:      []drivertype.Type{drivertype.Oracle},
			wantRecCount: 1,
			sinkFns:      []SinkTestFunc{assertSinkColDecimal(0, "7.25", nil)},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive
func TestQuery_expr_where_bool(t *testing.T) {
	// Note that there's no actual field .actor.is_alive, thus we're only testing
	// the query rendering, so skipExec=true. Unfortunately sakila doesn't have
	// a table with a bool column, so it's not trivial to test the query execution
	// given this test harness.
	testCases := []queryTestCase{
		{
			name:    "true",
			in:      `@sakila | .actor | where(.is_alive == true)`,
			wantSQL: `SELECT * FROM "actor" WHERE "is_alive" = true`,
			override: driverMap{
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "is_alive" = 1`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `is_alive` = true",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `is_alive` = true",
			},
			skipExec: true,
		},
		{
			name:    "false",
			in:      `@sakila | .actor | where(.is_alive == false)`,
			wantSQL: `SELECT * FROM "actor" WHERE "is_alive" = false`,
			override: driverMap{
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "is_alive" = 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `is_alive` = false",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `is_alive` = false",
			},
			skipExec: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.name), func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
