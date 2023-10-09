package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"github.com/neilotoole/sq/drivers/postgres"

	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

// TestQuery_func tests miscellaneous functions that aren't
// tested elsewhere.
//
//nolint:exhaustive
func TestQuery_func(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "max",
			in:           `@sakila | .actor | max(.actor_id)`,
			wantSQL:      `SELECT max("actor_id") AS "max(.actor_id)" FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT max(`actor_id`) AS `max(.actor_id)` FROM `actor`"},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "max(.actor_id)"),
				assertSinkColValue(0, int64(200)),
			},
		},
		{
			name:         "min",
			in:           `@sakila | .actor | min(.actor_id)`,
			wantSQL:      `SELECT min("actor_id") AS "min(.actor_id)" FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT min(`actor_id`) AS `min(.actor_id)` FROM `actor`"},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "min(.actor_id)"),
				assertSinkColValue(0, int64(1)),
			},
		},
		{
			name:         "avg",
			in:           `@sakila | .actor | avg(.actor_id)`,
			wantSQL:      `SELECT avg("actor_id") AS "avg(.actor_id)" FROM "actor"`,
			override:     driverMap{mysql.Type: "SELECT avg(`actor_id`) AS `avg(.actor_id)` FROM `actor`"},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "avg(.actor_id)"),

				// FIXME: The driver impls handle avg() differently. Some return
				// float64, some int, some decimal (string). The SLQ impl of avg()
				// needs to be modified to returned a consistent type.
				// assertSinkColValue(0, float64(100.5)),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}

func TestQuery_func_schema(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "sqlserver",
			in:           `@sakila | schema()`,
			wantSQL:      `SELECT SCHEMA_NAME() AS "schema()"`,
			onlyFor:      []source.DriverType{sqlserver.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, "dbo"),
			},
		},
		{
			name:         "postgres",
			in:           `@sakila | schema()`,
			wantSQL:      `SELECT current_schema() AS "schema()"`,
			onlyFor:      []source.DriverType{postgres.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, "public"),
			},
		},
		{
			name:         "mysql",
			in:           `@sakila | schema()`,
			wantSQL:      "SELECT DATABASE() AS `schema()`",
			onlyFor:      []source.DriverType{mysql.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, "sakila"),
			},
		},
		{
			name:         "sqlite",
			in:           `@sakila | schema()`,
			wantSQL:      `SELECT (SELECT name FROM pragma_database_list ORDER BY seq limit 1) AS "schema()"`,
			onlyFor:      []source.DriverType{sqlite3.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, "main"),
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			execQueryTestCase(t, tc)
		})
	}
}
