package libsq_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
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
			override:     driverMap{drivertype.TypeMy: "SELECT max(`actor_id`) AS `max(.actor_id)` FROM `actor`"},
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
			override:     driverMap{drivertype.TypeMy: "SELECT min(`actor_id`) AS `min(.actor_id)` FROM `actor`"},
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
			override:     driverMap{drivertype.TypeMy: "SELECT avg(`actor_id`) AS `avg(.actor_id)` FROM `actor`"},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMS},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMS},
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
			onlyFor:      []drivertype.Type{drivertype.TypePg},
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
			onlyFor:      []drivertype.Type{drivertype.TypePg},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMy},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMy},
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
			onlyFor:      []drivertype.Type{drivertype.TypeSL3},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMS},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMS},
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
			onlyFor:      []drivertype.Type{drivertype.TypePg},
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
			onlyFor:      []drivertype.Type{drivertype.TypePg},
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
			onlyFor:      []drivertype.Type{drivertype.TypeMy},
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
			onlyFor:      []drivertype.Type{drivertype.TypeSL3},
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

//nolint:lll,exhaustive
func TestQuery_func_rownum(t *testing.T) {
	testCases := []queryTestCase{
		{
			name:    "plain",
			in:      `@sakila | .actor | rownum()`,
			wantSQL: `SELECT (row_number() OVER (ORDER BY 1)) AS "rownum()" FROM "actor"`,
			override: driverMap{
				drivertype.TypeMS: `SELECT (row_number() OVER (ORDER BY (SELECT NULL))) AS "rownum()" FROM "actor"`,
				// We don't test the MySQL override because it uses a randomly generated variable value. E.g.
				//  SELECT (@row_number_dw5ch2ss:=@row_number_dw5ch2ss + 1) AS `rownum()` FROM `actor`
				drivertype.TypeMy: ``,
			},
			wantRecCount: 200,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "rownum()"),
				assertSinkCellValue(0, 0, int64(1)),
				assertSinkCellValue(199, 0, int64(200)),
			},
		},
		{
			name:    "plus_1",
			in:      `@sakila | .actor | rownum() + 1`,
			wantSQL: `SELECT (row_number() OVER (ORDER BY 1))+1 AS "rownum()+1" FROM "actor"`,
			override: driverMap{
				drivertype.TypeMS: `SELECT (row_number() OVER (ORDER BY (SELECT NULL)))+1 AS "rownum()+1" FROM "actor"`,
				drivertype.TypeMy: "",
			},
			wantRecCount: 200,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "rownum()+1"),
				assertSinkCellValue(0, 0, int64(2)),
				assertSinkCellValue(199, 0, int64(201)),
			},
		},
		{
			name:    "minus_1_alias",
			in:      `@sakila | .actor | (rownum()-1):zero_index`,
			wantSQL: `SELECT ((row_number() OVER (ORDER BY 1))-1) AS "zero_index" FROM "actor"`,
			override: driverMap{
				drivertype.TypeMS: `SELECT ((row_number() OVER (ORDER BY (SELECT NULL)))-1) AS "zero_index" FROM "actor"`,
				drivertype.TypeMy: "",
			},
			wantRecCount: 200,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "zero_index"),
				assertSinkCellValue(0, 0, int64(0)),
				assertSinkCellValue(199, 0, int64(199)),
			},
		},
		{
			name:         "column_orderby",
			in:           `@sakila | .actor | rownum(), .actor_id | order_by(.actor_id)`,
			wantSQL:      `SELECT (row_number() OVER (ORDER BY "actor_id")) AS "rownum()", "actor_id" FROM "actor" ORDER BY "actor_id"`,
			override:     driverMap{drivertype.TypeMy: ""},
			wantRecCount: 200,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "rownum()"),
				assertSinkCellValue(0, 0, int64(1)),
				assertSinkCellValue(199, 0, int64(200)),
			},
		},
		{
			name:         "double_invocation",
			in:           `@sakila | .actor | rownum():index1, .actor_id, rownum():index2 | order_by(.actor_id)`,
			wantSQL:      `SELECT (row_number() OVER (ORDER BY "actor_id")) AS "index1", "actor_id", (row_number() OVER (ORDER BY "actor_id")) AS "index2" FROM "actor" ORDER BY "actor_id"`,
			override:     driverMap{drivertype.TypeMy: ""},
			wantRecCount: 200,
			sinkFns: []SinkTestFunc{
				assertSinkColName(0, "index1"),
				assertSinkColName(2, "index2"),
				assertSinkCellValue(0, 0, int64(1)),
				assertSinkCellValue(0, 2, int64(1)),
				assertSinkCellValue(199, 0, int64(200)),
				assertSinkCellValue(199, 2, int64(200)),
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
