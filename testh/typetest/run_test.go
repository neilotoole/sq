package typetest_test

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/typetest"
)

// duckCases is a small case table that exercises the harness end to end
// against DuckDB, including a SQL-expression input. It is not DuckDB's
// column-type test; that belongs in the duckdb package.
var duckCases = []typetest.Case{
	{
		Name: "integer",
		Decl: "INTEGER",
		Zero: int64(0),
		Val:  int64(math.MaxInt32),
		Want: typetest.Want{
			BaseType:   typetest.Is("INTEGER"),
			ColumnType: typetest.Is("INTEGER"),
			Kind:       typetest.Is(kind.Int),
		},
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is("2147483647")}},
	},
	{
		Name: "varchar",
		Decl: "VARCHAR",
		Zero: "",
		Val:  "alfa",
		Want: typetest.Want{
			BaseType:   typetest.Is("VARCHAR"),
			ColumnType: typetest.Is("VARCHAR"),
			Kind:       typetest.Is(kind.Text),
		},
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is("'alfa'")}},
	},
	{
		Name: "decimal_5_2",
		Decl: "DECIMAL(5,2)",
		Zero: decimal.RequireFromString("0"),
		Val:  decimal.RequireFromString("999.99"),
		Want: typetest.Want{
			BaseType:   typetest.Is("DECIMAL(5,2)"),
			ColumnType: typetest.Is("DECIMAL(5,2)"),
			Kind:       typetest.Is(kind.Decimal),
		},
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is("999.99")}},
	},
	{
		Name: "sql_input",
		Decl: "VARCHAR",
		Zero: typetest.SQL("'x' || 'y'"),
		Val:  "alfa",
		Want: typetest.Want{
			BaseType:   typetest.Is("VARCHAR"),
			ColumnType: typetest.Is("VARCHAR"),
			Kind:       typetest.Is(kind.Text),
			Zero:       typetest.Is[any]("xy"),
		},
		PerVariant: typetest.PerVariant{Defaulted: typetest.Want{Default: typetest.Is("'alfa'")}},
	},
}

// TestRun_DuckDB runs the harness end to end against an embedded source, so
// that it's proven without an external database.
func TestRun_DuckDB(t *testing.T) {
	t.Parallel()

	typetest.Run(t, typetest.Config{
		Handle:    sakila.Duck,
		RowIDDecl: "INTEGER NOT NULL PRIMARY KEY",
		Cases:     duckCases,
	})
}

// TestRun_DuckDB_Dump runs the harness in dump mode, so that dumpActual runs
// under test. It can't be parallel: t.Setenv forbids it.
func TestRun_DuckDB_Dump(t *testing.T) {
	t.Setenv(typetest.EnvDump, "1")

	typetest.Run(t, typetest.Config{
		Handle:    sakila.Duck,
		RowIDDecl: "INTEGER NOT NULL PRIMARY KEY",
		Cases:     duckCases,
	})
}
