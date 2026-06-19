package duckdb

import (
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.DuckDB,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 1000,
		Ops:            dialect.DefaultOps(),
		ExecModeFor:    dialect.DefaultExecModeFor,
		Joins:          jointype.All(),
		Catalog:        true,
	}
}

// placeholders generates "$1, $2, ..., $n" style placeholders for n columns
// and m rows, matching DuckDB's (and Postgres's) preferred placeholder style.
func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)

	n := 1
	var sb strings.Builder
	for i := range numRows {
		sb.Reset()
		sb.WriteRune('(')
		for j := 1; j <= numCols; j++ {
			sb.WriteRune('$')
			sb.WriteString(strconv.Itoa(n))
			n++
			if j < numCols {
				sb.WriteString(driver.Comma)
			}
		}
		sb.WriteRune(')')
		rows[i] = sb.String()
	}

	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	// DuckDB uses the same schema/catalog function names as Postgres.
	r.FunctionNames[ast.FuncNameSchema] = "current_schema"
	r.FunctionNames[ast.FuncNameCatalog] = "current_database"
	// sum() is not given a SQL cast (unlike most drivers). DuckDB's native sum()
	// is already lossless for integer columns (widened to HUGEINT, surfaced as
	// decimal) and decimal columns (DECIMAL); a DECIMAL cast caps at precision 38
	// and would regress the HUGEINT integer range and round high-scale decimal
	// sums. But sum() over a DOUBLE column natively returns DOUBLE (kind.Float),
	// the lone non-decimal sum() across drivers. Pin the surfaced kind to decimal
	// instead of casting, so it matches every other driver: RecordMeta applies
	// the hint and the record munge coerces the float value to a decimal. The
	// value is still computed in float by DuckDB, so it can carry drift, the same
	// tradeoff accepted for sqlite3/rqlite. See #853 (and #839).
	r.FunctionResultKinds[ast.FuncNameSum] = kind.Decimal
	render.RegisterILikeFamily(r)
	return r
}
