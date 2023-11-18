package libsq_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/testh"

	"github.com/neilotoole/sq/drivers/sqlite3"

	"github.com/neilotoole/sq/drivers/postgres"

	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/drivers/mysql"

	_ "github.com/mattn/go-sqlite3"
)

const infoSchema = "information_schema"

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
				//
				// See also:
				// - https://github.com/golang/go/issues/30870
				// - https://github.com/golang-sql/decomposer

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
			name:         "sqlserver-default",
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
			name: "sqlserver-alt-no-change",
			// SQL Server doesn't support changing the schema on a per-connection
			// basis. So we expect the default schema to be returned.
			in: `@sakila | schema()`,
			beforeRun: func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext) {
				qc.Collection.Active().Schema = infoSchema
			},
			wantSQL:      `SELECT SCHEMA_NAME() AS "schema()"`,
			onlyFor:      []source.DriverType{sqlserver.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, "dbo"),
			},
		},
		{
			name:         "postgres-default",
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
			name: "postgres-alt",
			in:   `@sakila | schema()`,
			beforeRun: func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext) {
				qc.Collection.Active().Schema = infoSchema
			},
			wantSQL:      `SELECT current_schema() AS "schema()"`,
			onlyFor:      []source.DriverType{postgres.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, infoSchema),
			},
		},
		{
			name:         "mysql-default",
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
			name: "mysql-alt",
			in:   `@sakila | schema()`,
			beforeRun: func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext) {
				qc.Collection.Active().Schema = infoSchema
			},
			wantSQL:      "SELECT DATABASE() AS `schema()`",
			onlyFor:      []source.DriverType{mysql.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "schema()"),
				assertSinkColValue(0, infoSchema),
			},
		},
		{
			name:         "sqlite-default",
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

func TestQuery_func_catalog(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:         "sqlserver-default",
			in:           `@sakila | catalog()`,
			wantSQL:      `SELECT DB_NAME() AS "catalog()"`,
			onlyFor:      []source.DriverType{sqlserver.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "sakila"),
			},
		},
		{
			name: "sqlserver-alt",
			in:   `@sakila | catalog()`,
			beforeRun: func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext) {
				qc.Collection.Active().Catalog = "model"
			},
			wantSQL:      `SELECT DB_NAME() AS "catalog()"`,
			onlyFor:      []source.DriverType{sqlserver.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "model"),
			},
		},
		{
			name:         "postgres-default",
			in:           `@sakila | catalog()`,
			wantSQL:      `SELECT current_database() AS "catalog()"`,
			onlyFor:      []source.DriverType{postgres.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "sakila"),
			},
		},
		{
			name: "postgres-alt",
			in:   `@sakila | catalog()`,
			beforeRun: func(tc queryTestCase, th *testh.Helper, qc *libsq.QueryContext) {
				qc.Collection.Active().Catalog = "postgres"
			},
			wantSQL:      `SELECT current_database() AS "catalog()"`,
			onlyFor:      []source.DriverType{postgres.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "postgres"),
			},
		},
		{
			name:         "mysql",
			in:           `@sakila | catalog()`,
			wantSQL:      "SELECT (SELECT CATALOG_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = DATABASE() LIMIT 1) AS `catalog()`", //nolint:lll
			onlyFor:      []source.DriverType{mysql.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "def"),
			},
		},
		{
			name: "sqlite",
			in:   `@sakila | catalog()`,
			// SQLite doesn't support catalogs, so we (somewhat arbitrarily)
			// return the string "default". This behavior may change
			// upon feedback.
			wantSQL:      `SELECT (SELECT 'default') AS "catalog()"`,
			onlyFor:      []source.DriverType{sqlite3.Type},
			wantRecCount: 1,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "catalog()"),
				assertSinkColValue(0, "default"),
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
