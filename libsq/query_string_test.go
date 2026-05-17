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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'AN') > 0",
			},
			skipExec: true,
		},
		{
			// Pair test: uppercase pattern matches both ANGELA rows in sakila
			// (ANGELA HUDSON, ANGELA WITHERSPOON).
			name:    "contains/case-sensitive-uppercase",
			in:      `@sakila | .actor | where(contains(.first_name, "ANGELA"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%ANGELA%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'ANGELA') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%ANGELA%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'ANGELA') > 0",
			},
			wantRecCount: 2,
		},
		{
			// Pair test: lowercase pattern matches zero rows on all drivers
			// because sakila first_names are stored UPPERCASE. This is the
			// behavioural assertion that proves case sensitivity.
			name:    "contains/case-sensitive-lowercase-no-match",
			in:      `@sakila | .actor | where(contains(.first_name, "angela"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%angela%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", 'angela') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%angela%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%angela%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'angela') > 0",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, '50%') > 0",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'a_b') > 0",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'a|b') > 0",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`last_name`, 'O''B') > 0",
			},
			wantRecCount: 0,
		},
		{
			// SLQ STRING tokens support JSON-style backslash escapes (see
			// grammar/SLQ.g4 STRING/ESC). The renderer decodes them via
			// unquoteLiteral so `"\n"` is a real newline in the search
			// pattern, not the two characters `\` + `n`. Sakila first_names
			// contain no embedded newlines, so the count is 0.
			name:    "contains/decodes-newline-escape",
			in:      `@sakila | .actor | where(contains(.first_name, "a\nb"))`,
			wantSQL: "SELECT * FROM \"actor\" WHERE \"first_name\" LIKE '%a\nb%' ESCAPE '|'",
			override: driverMap{
				drivertype.SQLite:     "SELECT * FROM \"actor\" WHERE instr(\"first_name\", 'a\nb') > 0",
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%a\nb%' ESCAPE '|'",
				drivertype.MSSQL:      "SELECT * FROM \"actor\" WHERE \"first_name\" COLLATE Latin1_General_BIN2 LIKE '%a\nb%' ESCAPE '|'",
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, 'a\nb') > 0",
			},
			wantRecCount: 0,
		},
		{
			// Empty pattern matches every non-NULL row on all drivers.
			// SQLite's instr-based path already returns this naturally:
			// instr(col, '') = 1, so the `> 0` test is true for any
			// non-NULL row. Tested here to lock in the cross-driver
			// invariant alongside startswith/endswith.
			name:    "contains/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(contains(.first_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%%' ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" LIKE '%%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", '') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, '') > 0",
			},
			wantRecCount: 200,
		},
		{
			// SQL Server's LIKE treats `[...]` as a character-class
			// wildcard (e.g. `[A-Z]` matches any uppercase letter). The
			// SQL Server renderer escapes `[` and `]` so the literal
			// substring semantics hold. Sakila first_names contain no
			// literal `[A-Z]` substring, so the expected count is 0; if
			// bracket escaping regressed, this query would match every
			// row whose first_name has an uppercase letter (all 200).
			name:    "contains/sqlserver-bracket-class-is-literal",
			in:      `@sakila | .actor | where(contains(.first_name, "[A-Z]"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '%[A-Z]%' ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" LIKE '%[A-Z]%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE instr("first_name", '[A-Z]') > 0`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '%[A-Z]%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '%|[A-Z|]%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE position(`first_name`, '[A-Z]') > 0",
			},
			wantRecCount: 0,
		},
		{
			name:            "contains/wrong-arg-count",
			in:              `@sakila | .actor | where(contains(.first_name))`,
			wantErrContains: "contains() requires exactly 2 arguments",
		},
		{
			name:            "contains/non-literal-pattern",
			in:              `@sakila | .actor | where(contains(.first_name, .last_name))`,
			wantErrContains: "contains() second argument must be a string literal",
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
			// Pair test: uppercase prefix "GU" matches 3 sakila last_names
			// (GUINESS, GUARDINO, GUERIN). Executed to prove the renderer
			// actually matches non-empty prefixes — without a row-count
			// assertion, a renderer that never matched any prefix would
			// still pass the lowercase no-match case below.
			name:    "startswith/basic-uppercase",
			in:      `@sakila | .actor | where(startswith(.last_name, "GU"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE 'GU%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", 1, 2) = 'GU'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY 'GU%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE 'GU%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWith(`last_name`, 'GU')",
			},
			wantRecCount: 3,
		},
		{
			name:    "startswith/case-sensitive-lowercase-no-match",
			in:      `@sakila | .actor | where(startswith(.last_name, "gu"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE 'gu%' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", 1, 2) = 'gu'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY 'gu%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE 'gu%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWith(`last_name`, 'gu')",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWith(`first_name`, '50%')",
			},
			wantRecCount: 0,
		},
		{
			// Empty pattern matches every non-NULL row on all drivers.
			// SQLite's substr(col, 1, 0) returns '', which equals ''
			// naturally — no special-case needed (unlike endswith).
			name:    "startswith/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(startswith(.last_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", 1, 0) = ''`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWith(`last_name`, '')",
			},
			wantRecCount: 200,
		},
		{
			name:            "startswith/wrong-arg-count",
			in:              `@sakila | .actor | where(startswith(.last_name))`,
			wantErrContains: "startswith() requires exactly 2 arguments",
		},
		{
			name:            "startswith/non-literal-pattern",
			in:              `@sakila | .actor | where(startswith(.last_name, .first_name))`,
			wantErrContains: "startswith() second argument must be a string literal",
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
			// Pair test: uppercase suffix "SON" matches 9 sakila last_names
			// (JACKSON, JOHANSSON, NEESON, DAVIS-SON variants, etc.).
			// Executed to prove the renderer actually matches non-empty
			// suffixes — without this assertion, a renderer that never
			// matched any suffix would still pass the lowercase no-match
			// case below.
			name:    "endswith/basic-uppercase",
			in:      `@sakila | .actor | where(endswith(.last_name, "SON"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%SON' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", -3) = 'SON'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%SON' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%SON' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWith(`last_name`, 'SON')",
			},
			wantRecCount: 9,
		},
		{
			name:    "endswith/case-sensitive-lowercase-no-match",
			in:      `@sakila | .actor | where(endswith(.last_name, "son"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%son' ESCAPE '|'`,
			override: driverMap{
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE substr("last_name", -3) = 'son'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%son' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%son' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWith(`last_name`, 'son')",
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
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWith(`first_name`, '_x')",
			},
			wantRecCount: 0,
		},
		{
			// Empty pattern matches every non-NULL row on all drivers.
			// SQLite's substr-based path would naively be false for every
			// row (substr(col, -0) returns the full string); the renderer
			// special-cases this to `col LIKE '%'` so NULL handling matches
			// the LIKE-based drivers (including under negation, where
			// `col IS NOT NULL` would diverge). See
			// drivers/sqlite3/render.go:renderFuncEndsWithSubstr.
			name:    "endswith/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(endswith(.last_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE '%'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `last_name` LIKE BINARY '%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_BIN2 LIKE '%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWith(`last_name`, '')",
			},
			wantRecCount: 200,
		},
		{
			name:            "endswith/wrong-arg-count",
			in:              `@sakila | .actor | where(endswith(.last_name))`,
			wantErrContains: "endswith() requires exactly 2 arguments",
		},
		{
			name:            "endswith/non-literal-pattern",
			in:              `@sakila | .actor | where(endswith(.last_name, .first_name))`,
			wantErrContains: "endswith() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_icontains(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Pair test: lowercase pattern matches 2 ANGELA rows in
			// sakila despite the data being stored UPPERCASE. This
			// proves case-insensitivity is wired through correctly.
			name:    "icontains/case-insensitive-lowercase-match",
			in:      `@sakila | .actor | where(icontains(.first_name, "angela"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('%angela%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '%angela%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '%angela%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '%angela%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('%angela%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '%angela%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE positionCaseInsensitive(`first_name`, 'angela') > 0",
			},
			wantRecCount: 2,
		},
		{
			// Pair test: uppercase pattern also matches the same 2 rows
			// — case is ignored on both sides.
			name:    "icontains/case-insensitive-uppercase-match",
			in:      `@sakila | .actor | where(icontains(.first_name, "ANGELA"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('%ANGELA%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('%ANGELA%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '%ANGELA%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE positionCaseInsensitive(`first_name`, 'ANGELA') > 0",
			},
			wantRecCount: 2,
		},
		{
			// Pair test: pattern that exists nowhere returns 0 rows.
			name:         "icontains/no-match",
			in:           `@sakila | .actor | where(icontains(.first_name, "xyzzy"))`,
			wantRecCount: 0,
		},
		{
			// Escape semantics still apply (% is auto-escaped) — same as
			// contains. Sakila first_names contain no literal %.
			name:    "icontains/escapes-percent",
			in:      `@sakila | .actor | where(icontains(.first_name, "50%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('%50|%%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '%50|%%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '%50|%%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '%50|%%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('%50|%%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '%50|%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE positionCaseInsensitive(`first_name`, '50%') > 0",
			},
			wantRecCount: 0,
		},
		{
			name:    "icontains/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(icontains(.first_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('%%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '%%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '%%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '%%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('%%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE positionCaseInsensitive(`first_name`, '') > 0",
			},
			wantRecCount: 200,
		},
		{
			name:            "icontains/wrong-arg-count",
			in:              `@sakila | .actor | where(icontains(.first_name))`,
			wantErrContains: "icontains() requires exactly 2 arguments",
		},
		{
			name:            "icontains/non-literal-pattern",
			in:              `@sakila | .actor | where(icontains(.first_name, .last_name))`,
			wantErrContains: "icontains() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_istartswith(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Pair test: lowercase prefix matches 3 sakila last_names
			// (GUINESS, GUARDINO, GUERIN) despite storage being UPPERCASE.
			name:    "istartswith/case-insensitive-lowercase-match",
			in:      `@sakila | .actor | where(istartswith(.last_name, "gu"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('gu%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE 'gu%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE 'gu%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE 'gu%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('gu%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE 'gu%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWithCaseInsensitive(`last_name`, 'gu')",
			},
			wantRecCount: 3,
		},
		{
			// Pair test: uppercase prefix matches the same 3 rows.
			name:    "istartswith/case-insensitive-uppercase-match",
			in:      `@sakila | .actor | where(istartswith(.last_name, "GU"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('GU%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE 'GU%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE 'GU%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE 'GU%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('GU%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE 'GU%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWithCaseInsensitive(`last_name`, 'GU')",
			},
			wantRecCount: 3,
		},
		{
			// Pair test: prefix that exists nowhere returns 0 rows.
			name:         "istartswith/no-match",
			in:           `@sakila | .actor | where(istartswith(.last_name, "xyzzy"))`,
			wantRecCount: 0,
		},
		{
			// Escape semantics still apply (% is auto-escaped).
			name:    "istartswith/escapes-percent",
			in:      `@sakila | .actor | where(istartswith(.first_name, "50%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('50|%%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '50|%%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '50|%%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '50|%%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('50|%%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '50|%%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE startsWithCaseInsensitive(`first_name`, '50%')",
			},
			wantRecCount: 0,
		},
		{
			name:    "istartswith/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(istartswith(.last_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE '%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE '%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE '%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE length(`last_name`) >= 0",
			},
			wantRecCount: 200,
		},
		{
			name:            "istartswith/wrong-arg-count",
			in:              `@sakila | .actor | where(istartswith(.last_name))`,
			wantErrContains: "istartswith() requires exactly 2 arguments",
		},
		{
			name:            "istartswith/non-literal-pattern",
			in:              `@sakila | .actor | where(istartswith(.last_name, .first_name))`,
			wantErrContains: "istartswith() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_iendswith(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Pair test: lowercase suffix matches 9 sakila last_names
			// (JACKSON, JOHANSSON, NEESON, …) despite storage being UPPERCASE.
			name:    "iendswith/case-insensitive-lowercase-match",
			in:      `@sakila | .actor | where(iendswith(.last_name, "son"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('%son') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE '%son' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE '%son' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE '%son' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('%son') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE '%son' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWithCaseInsensitive(`last_name`, 'son')",
			},
			wantRecCount: 9,
		},
		{
			// Pair test: uppercase suffix matches the same 9 rows.
			name:    "iendswith/case-insensitive-uppercase-match",
			in:      `@sakila | .actor | where(iendswith(.last_name, "SON"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('%SON') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE '%SON' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE '%SON' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE '%SON' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('%SON') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE '%SON' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWithCaseInsensitive(`last_name`, 'SON')",
			},
			wantRecCount: 9,
		},
		{
			// Pair test: suffix that exists nowhere returns 0 rows.
			name:         "iendswith/no-match",
			in:           `@sakila | .actor | where(iendswith(.last_name, "xyzzy"))`,
			wantRecCount: 0,
		},
		{
			// Escape semantics still apply (_ is auto-escaped).
			name:    "iendswith/escapes-underscore",
			in:      `@sakila | .actor | where(iendswith(.first_name, "_x"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('%|_x') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '%|_x' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '%|_x' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '%|_x' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('%|_x') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '%|_x' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE endsWithCaseInsensitive(`first_name`, '_x')",
			},
			wantRecCount: 0,
		},
		{
			// Empty pattern matches every non-NULL row. ClickHouse special-cases
			// this to length(col) >= 0 because endsWithCaseInsensitive(col, '')
			// returns false in ClickHouse (unlike the case-sensitive endsWith),
			// and length(NULL) = NULL so the expression NULL-propagates correctly
			// under negation.
			name:    "iendswith/empty-pattern-matches-all",
			in:      `@sakila | .actor | where(iendswith(.last_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("last_name") LIKE LOWER('%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "last_name" ILIKE '%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "last_name" ILIKE '%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "last_name" LIKE '%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`last_name`) LIKE LOWER('%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "last_name" COLLATE Latin1_General_CI_AS LIKE '%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE length(`last_name`) >= 0",
			},
			wantRecCount: 200,
		},
		{
			// NULL-propagation regression: in the SQLite sakila, all 603
			// address rows have NULL address2. With correct NULL
			// propagation, iendswith(NULL, "") = NULL, which is excluded
			// by WHERE, so the result must be 0 rows. A buggy
			// implementation that uses col IS NOT NULL instead of
			// length(col) >= 0 on ClickHouse would return all rows under
			// negation; the ClickHouse SQL shape for the empty-pattern
			// case is separately pinned by iendswith/empty-pattern-matches-all.
			name:         "iendswith/null-column-empty-pattern-propagates-null",
			in:           `@sakila | .address | where(iendswith(.address2, ""))`,
			onlyFor:      []drivertype.Type{drivertype.SQLite},
			wantRecCount: 0,
		},
		{
			name:            "iendswith/wrong-arg-count",
			in:              `@sakila | .actor | where(iendswith(.last_name))`,
			wantErrContains: "iendswith() requires exactly 2 arguments",
		},
		{
			name:            "iendswith/non-literal-pattern",
			in:              `@sakila | .actor | where(iendswith(.last_name, .first_name))`,
			wantErrContains: "iendswith() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_like(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Basic wildcard prefix matches 4 PENELOPE rows on all CS
			// drivers. On SQLite (ASCII-CI), the same query matches the
			// same 4 rows (see SQLite-quirk pair test below).
			name:    "like/wildcard-prefix-uppercase",
			in:      `@sakila | .actor | where(like(.first_name, "PEN%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE 'PEN%' ESCAPE '|'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY 'PEN%' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE 'PEN%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE 'PEN%'",
			},
			wantRecCount: 4,
		},
		{
			// SQLite-specific quirk: `like` is ASCII-CI by default, so
			// a lowercase pattern matches uppercase data.
			name:         "like/sqlite-ascii-ci-quirk",
			in:           `@sakila | .actor | where(like(.first_name, "pen%"))`,
			wantSQL:      `SELECT * FROM "actor" WHERE "first_name" LIKE 'pen%' ESCAPE '|'`,
			onlyFor:      []drivertype.Type{drivertype.SQLite},
			wantRecCount: 4,
		},
		{
			// Companion to like/sqlite-ascii-ci-quirk: on case-sensitive
			// drivers, lowercase pattern matches zero rows because data is
			// stored UPPERCASE. SQLite is excluded via onlyFor because its
			// default LIKE is ASCII-CI (covered by the sqlite-ascii-ci-quirk
			// test above).
			name:         "like/wildcard-prefix-lowercase-non-sqlite-no-match",
			in:           `@sakila | .actor | where(like(.first_name, "pen%"))`,
			onlyFor:      []drivertype.Type{drivertype.Pg, drivertype.DuckDB, drivertype.MySQL, drivertype.MSSQL, drivertype.ClickHouse, drivertype.Oracle},
			wantRecCount: 0,
		},
		{
			// `PEN_LOPE` has a single-char wildcard (`_`) in position 4,
			// matching the 4 PENELOPE rows in sakila. Demonstrates that
			// `_` is treated as a wildcard, not auto-escaped.
			name:    "like/single-char-wildcard",
			in:      `@sakila | .actor | where(like(.first_name, "PEN_LOPE"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE 'PEN_LOPE' ESCAPE '|'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY 'PEN_LOPE' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE 'PEN_LOPE' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE 'PEN_LOPE'",
			},
			wantRecCount: 4,
		},
		{
			// Empty pattern matches only empty strings (real LIKE
			// semantics), unlike contains/icontains which match every
			// non-NULL row. Sakila has no empty first_names → 0 rows.
			name:    "like/empty-pattern-matches-empty-strings-only",
			in:      `@sakila | .actor | where(like(.first_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE '' ESCAPE '|'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY '' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE '' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE ''",
			},
			wantRecCount: 0,
		},
		{
			// Pin the SQL shape for `|` in the pattern. The engine reserves
			// `|` as the ESCAPE character on every driver except ClickHouse,
			// which omits the ESCAPE clause entirely. Runtime behavior of
			// a bare `|` diverges (silently dropped on lenient drivers,
			// "invalid escape sequence" runtime error on Postgres/Oracle),
			// so we don't exec — see the v1-limitation note in the docs.
			name:    "like/escape-char-in-pattern-pins-shape",
			in:      `@sakila | .actor | where(like(.first_name, "a|b"))`,
			wantSQL: `SELECT * FROM "actor" WHERE "first_name" LIKE 'a|b' ESCAPE '|'`,
			override: driverMap{
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE `first_name` LIKE BINARY 'a|b' ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_BIN2 LIKE 'a|b' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` LIKE 'a|b'",
			},
			skipExec: true,
		},
		{
			name:            "like/wrong-arg-count",
			in:              `@sakila | .actor | where(like(.first_name))`,
			wantErrContains: "like() requires exactly 2 arguments",
		},
		{
			name:            "like/non-literal-pattern",
			in:              `@sakila | .actor | where(like(.first_name, .last_name))`,
			wantErrContains: "like() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

//nolint:exhaustive,lll
func TestQuery_string_ilike(t *testing.T) {
	testCases := []queryTestCase{
		{
			// Pair test: lowercase wildcard pattern matches 4 PENELOPEs
			// on every driver, demonstrating case-insensitive wildcard
			// matching.
			name:    "ilike/wildcard-prefix-lowercase-match",
			in:      `@sakila | .actor | where(ilike(.first_name, "pen%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('pen%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE 'pen%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE 'pen%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE 'pen%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('pen%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE 'pen%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` ILIKE 'pen%'",
			},
			wantRecCount: 4,
		},
		{
			// Pair test: uppercase wildcard pattern also matches the
			// same 4 PENELOPEs.
			name:    "ilike/wildcard-prefix-uppercase-match",
			in:      `@sakila | .actor | where(ilike(.first_name, "PEN%"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('PEN%') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE 'PEN%' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE 'PEN%' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE 'PEN%' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('PEN%') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE 'PEN%' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` ILIKE 'PEN%'",
			},
			wantRecCount: 4,
		},
		{
			// No-match: pattern that exists nowhere returns 0 rows.
			name:         "ilike/no-match",
			in:           `@sakila | .actor | where(ilike(.first_name, "xyzzy%"))`,
			wantRecCount: 0,
		},
		{
			// User-controlled `_` wildcard, mixed case — pen_lope matches
			// 4 PENELOPEs on every driver under CI.
			name:    "ilike/single-char-wildcard-mixed-case",
			in:      `@sakila | .actor | where(ilike(.first_name, "pen_LOPE"))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('pen_LOPE') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE 'pen_LOPE' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE 'pen_LOPE' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE 'pen_LOPE' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('pen_LOPE') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE 'pen_LOPE' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` ILIKE 'pen_LOPE'",
			},
			wantRecCount: 4,
		},
		{
			// Empty pattern matches only empty strings (real LIKE
			// semantics). Sakila has no empty first_names → 0 rows.
			name:    "ilike/empty-pattern-matches-empty-strings-only",
			in:      `@sakila | .actor | where(ilike(.first_name, ""))`,
			wantSQL: `SELECT * FROM "actor" WHERE LOWER("first_name") LIKE LOWER('') ESCAPE '|'`,
			override: driverMap{
				drivertype.Pg:         `SELECT * FROM "actor" WHERE "first_name" ILIKE '' ESCAPE '|'`,
				drivertype.DuckDB:     `SELECT * FROM "actor" WHERE "first_name" ILIKE '' ESCAPE '|'`,
				drivertype.SQLite:     `SELECT * FROM "actor" WHERE "first_name" LIKE '' ESCAPE '|'`,
				drivertype.MySQL:      "SELECT * FROM `actor` WHERE LOWER(`first_name`) LIKE LOWER('') ESCAPE '|'",
				drivertype.MSSQL:      `SELECT * FROM "actor" WHERE "first_name" COLLATE Latin1_General_CI_AS LIKE '' ESCAPE '|'`,
				drivertype.ClickHouse: "SELECT * FROM `actor` WHERE `first_name` ILIKE ''",
			},
			wantRecCount: 0,
		},
		{
			name:            "ilike/wrong-arg-count",
			in:              `@sakila | .actor | where(ilike(.first_name))`,
			wantErrContains: "ilike() requires exactly 2 arguments",
		},
		{
			name:            "ilike/non-literal-pattern",
			in:              `@sakila | .actor | where(ilike(.first_name, .last_name))`,
			wantErrContains: "ilike() second argument must be a string literal",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
