package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq/source/drivertype"
)

//nolint:exhaustive,lll
func TestQuery_string_contains(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Case-sensitive: uppercase 'AN' matches several sakila first_names
			// (ANGELA, ANGELINA, DAN, EVAN, ...). We assert SQL shape; row count
			// is asserted by the case-sensitive-pair tests below.
			name:    "contains/basic",
			in:      `@sakila | .actor | where(contains(.first_name, "AN"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%AN%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'AN') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%AN%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%AN%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%AN%' ESCAPE '|'",
			},
			skipExec: true,
		},
		{
			// Pair test: uppercase pattern matches both ANGELA rows in sakila
			// (ANGELA HUDSON, ANGELA WITHERSPOON).
			name:         "contains/case-sensitive-uppercase",
			in:           `@sakila | .actor | where(contains(.first_name, "ANGELA"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" LIKE '%ANGELA%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'ANGELA') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%ANGELA%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%ANGELA%' ESCAPE '|'",
			},
			wantRecCount: 2,
		},
		{
			// Pair test: lowercase pattern matches zero rows on all drivers
			// because sakila first_names are stored UPPERCASE. This is the
			// behavioural assertion that proves case sensitivity.
			name:         "contains/case-sensitive-lowercase-no-match",
			in:           `@sakila | .actor | where(contains(.first_name, "angela"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" LIKE '%angela%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'angela') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%angela%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%angela%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%angela%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "contains/escapes-percent",
			in:      `@sakila | .actor | where(contains(.first_name, "50%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%50|%%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", '50%') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%50|%%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%50|%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%50|%%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "contains/escapes-underscore",
			in:      `@sakila | .actor | where(contains(.first_name, "a_b"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%a|_b%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'a_b') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%a|_b%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%a|_b%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%a|_b%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "contains/escapes-pipe",
			in:      `@sakila | .actor | where(contains(.first_name, "a|b"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%a||b%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'a|b') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%a||b%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%a||b%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%a||b%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "contains/escapes-single-quote",
			in:      `@sakila | .actor | where(contains(.last_name, "O'B"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%O''B%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("last_name", 'O''B') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%O''B%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%O''B%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `last_name` LIKE '%O''B%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "contains/wrong-arg-count",
			in:      `@sakila | .actor | where(contains(.first_name))`,
			wantErr: true,
		},
		{
			name:    "contains/non-literal-pattern",
			in:      `@sakila | .actor | where(contains(.first_name, .last_name))`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_startswith(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "startswith/basic-uppercase",
			in:      `@sakila | .actor | where(startswith(.last_name, "GU"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE 'GU%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", 1, 2) = 'GU'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY 'GU%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE 'GU%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `last_name` LIKE 'GU%' ESCAPE '|'",
			},
			skipExec: true,
		},
		{
			name:         "startswith/case-sensitive-lowercase-no-match",
			in:           `@sakila | .actor | where(startswith(.last_name, "gu"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "last_name" LIKE 'gu%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", 1, 2) = 'gu'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY 'gu%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE 'gu%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `last_name` LIKE 'gu%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "startswith/escapes-percent",
			in:      `@sakila | .actor | where(startswith(.first_name, "50%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '50|%%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("first_name", 1, 3) = '50%'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '50|%%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '50|%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '50|%%' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_endswith(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "endswith/basic-uppercase",
			in:      `@sakila | .actor | where(endswith(.last_name, "SON"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%SON' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", -3) = 'SON'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%SON' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%SON' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `last_name` LIKE '%SON' ESCAPE '|'",
			},
			skipExec: true,
		},
		{
			name:         "endswith/case-sensitive-lowercase-no-match",
			in:           `@sakila | .actor | where(endswith(.last_name, "son"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "last_name" LIKE '%son' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", -3) = 'son'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%son' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%son' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `last_name` LIKE '%son' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
		{
			name:    "endswith/escapes-underscore",
			in:      `@sakila | .actor | where(endswith(.first_name, "_x"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%|_x' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("first_name", -2) = '_x'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%|_x' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%|_x' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE '%|_x' ESCAPE '|'",
			},
			wantRecCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
