package duckdb

import (
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/jointype"
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
	render.RegisterILikeFamily(r)
	return r
}
