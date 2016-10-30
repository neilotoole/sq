package engine

import (
	"fmt"

	"strings"

	"math"

	"reflect"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/drvr"
)

var renderers = make(map[drvr.Type]Renderer)

// Renderer renders driver-specific SQL fragments.
type Renderer interface {
	FromTable(tblSel *ast.TblSelector) (string, error)
	SelectCols(cols []ast.ColExpr) (string, error)
	SelectAll(tblSel *ast.TblSelector) (string, error)
	Range(rr *ast.RowRange) (string, error)
	Join(fnJoin *ast.Join) (string, error)
	CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error)
	CreateInsertStmt(tblName string, colNames []string) (string, error)
}

func init() {
	renderers[drvr.Type("mysql")] = &MySQLRenderer{rndr: &baseRenderer{quote: "`", colQuote: "`"}}
	renderers[drvr.Type("postgres")] = &PostgresRenderer{rndr: &baseRenderer{quote: `"`}}
	renderers[drvr.Type("sqlite3")] = &SQLite3Renderer{rndr: &baseRenderer{quote: `"`}}
	renderers[drvr.Type("xlsx")] = &XLSXRenderer{rndr: &baseRenderer{quote: `"`}}

	csv := &CSVRenderer{rndr: &baseRenderer{quote: `"`}}
	renderers[drvr.Type("csv")] = csv
	renderers[drvr.Type("tsv")] = csv
}

// RendererFor returns a Renderer for the given driver type.
func RendererFor(typ drvr.Type) (Renderer, error) {

	r, ok := renderers[typ]
	if !ok {
		return nil, errorf("renderer for driver %q not found", typ)
	}

	return r, nil
}

type baseRenderer struct {
	// quote is the driver-specific quote rune, e.g. " or `
	quote    string
	colQuote string
}

func (r *baseRenderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	sql := fmt.Sprintf("SELECT * FROM %v%s%v", r.quote, tblSel.SelValue(), r.quote)
	lg.Debugf("returning SQL fragment: %s", sql)
	return sql, nil
}
func (r *baseRenderer) FromTable(tblSel *ast.TblSelector) (string, error) {

	tblName := tblSel.SelValue()
	if tblName == "" {
		return "", errorf("selector has empty table name: %q", tblSel.Text())
	}

	clause := fmt.Sprintf("FROM %v%s%v", r.quote, tblSel.SelValue(), r.quote)
	lg.Debugf("returning SQL fragment: %s", clause)
	return clause, nil
}

func (r *baseRenderer) Join(fnJoin *ast.Join) (string, error) {

	joinType := "INNER JOIN"

	onClause := ""

	if len(fnJoin.Children()) == 0 {
		joinType = "NATURAL JOIN"
	} else {
		joinExpr, ok := fnJoin.Children()[0].(*ast.JoinConstraint)
		if !ok {
			return "", errorf("expected *FnJoinExpr but got %T", fnJoin.Children()[0])
		}

		leftOperand := ""
		operator := ""
		rightOperand := ""

		if len(joinExpr.Children()) == 1 {
			// It's a single col selector
			colSel, ok := joinExpr.Children()[0].(*ast.ColSelector)
			if !ok {
				return "", errorf("expected *ColSelector but got %T", joinExpr.Children()[0])
			}

			leftOperand = fmt.Sprintf("%s%s%s.%s%s%s", r.quote, fnJoin.LeftTbl().SelValue(), r.quote, r.quote, colSel.SelValue(), r.quote)
			operator = "=="
			rightOperand = fmt.Sprintf("%s%s%s.%s%s%s", r.quote, fnJoin.RightTbl().SelValue(), r.quote, r.quote, colSel.SelValue(), r.quote)
		} else {
			leftOperand = joinExpr.Children()[0].Text()[1:]
			operator = joinExpr.Children()[1].Text()
			rightOperand = joinExpr.Children()[2].Text()[1:]
		}

		if operator == "==" {
			operator = "="
		}

		onClause = fmt.Sprintf(" ON %s %s %s", leftOperand, operator, rightOperand)
	}

	sql := fmt.Sprintf("FROM %s%s%s %s %s%s%s", r.quote, fnJoin.LeftTbl().SelValue(), r.quote, joinType, r.quote, fnJoin.RightTbl().SelValue(), r.quote)
	if onClause != "" {
		sql = sql + " " + onClause
	}

	lg.Debugf("returning JOIN fragment: %s", sql)
	return sql, nil
}

func (r *baseRenderer) Range(rr *ast.RowRange) (string, error) {

	if rr == nil {
		return "", nil
	}

	if rr.Limit < 0 && rr.Offset < 0 {
		return "", nil
	}

	limit := ""
	offset := ""
	if rr.Limit > -1 {
		limit = fmt.Sprintf(" LIMIT %d", rr.Limit)
	}
	if rr.Offset > -1 {
		offset = fmt.Sprintf(" OFFSET %d", rr.Offset)

		if rr.Limit == -1 {
			// MySQL requires a LIMIT if OFFSET is used. Therefore
			// we make the LIMIT a very large number
			limit = fmt.Sprintf(" LIMIT %d", math.MaxInt64)
		}
	}

	sql := limit + offset
	lg.Debugf("returning SQL fragment: %s", sql)

	return sql, nil
}

func (r *baseRenderer) SelectCols(cols []ast.ColExpr) (string, error) {
	lg.Debugf("generating select clause for cols: %v", cols)

	if len(cols) == 0 {
		return "SELECT *", nil
	}

	vals := make([]string, len(cols))

	for i, col := range cols {
		colText, err := col.ColExpr()
		if err != nil {
			return "", errorf("unable to extract col expr from %q: %v", col, err)
		}
		if strings.IndexRune(colText, '.') == -1 {

			// it's a regular (non-scoped) col name, e.g. "uid"
			vals[i] = fmt.Sprintf("%s%s%s", r.quote, colText, r.quote)
			continue
		}

		// the expr contains a period, so it's likely scoped, e.g. "user.uid"
		parts := strings.Split(colText, ".")
		if len(parts) != 2 {
			return "", errorf("expected scoped col expr %q to have 2 parts, but got: %v", col, parts)
		}

		vals[i] = fmt.Sprintf("%s%s%s.%s%s%s", r.quote, parts[0], r.quote, r.quote, parts[1], r.quote)
	}

	text := "SELECT " + strings.Join(vals, ", ")
	return text, nil
}

type MySQLRenderer struct {
	rndr *baseRenderer
}

func (r *MySQLRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *MySQLRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *MySQLRenderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *MySQLRenderer) FromTable(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *MySQLRenderer) Join(fnJoin *ast.Join) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *MySQLRenderer) Range(rr *ast.RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *MySQLRenderer) SelectCols(cols []ast.ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type PostgresRenderer struct {
	rndr *baseRenderer
}

func (r *PostgresRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *PostgresRenderer) FromTable(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *PostgresRenderer) Join(fnJoin *ast.Join) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *PostgresRenderer) Range(rr *ast.RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *PostgresRenderer) SelectCols(cols []ast.ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type SQLite3Renderer struct {
	rndr *baseRenderer
}

func (r *SQLite3Renderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {

	escapedColNames := make([]string, len(colNames))
	for i, colName := range colNames {
		escapedColNames[i] = r.rndr.quote + colName + r.rndr.quote
	}

	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = "?"
	}

	insertTpl := fmt.Sprintf(`INSERT INTO %s%%s%s ( %%s ) VALUES ( %%s )`, r.rndr.quote, r.rndr.quote)
	lg.Debugf("insertTpl: %s", insertTpl)

	insertStmt := fmt.Sprintf(insertTpl, tblName, strings.Join(escapedColNames, ", "), strings.Join(placeholders, ", "))

	return insertStmt, nil
}

func (r *SQLite3Renderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {

	lg.Debugf("table name: %s | col names: %v", tblName, colNames)
	colExprs := make([]string, len(colNames))
	for i := 0; i < len(colNames); i++ {
		typ := r.getNativeType(colTypes[i])
		colExprs[i] = fmt.Sprintf(`%s%s%s %s`, r.rndr.quote, colNames[i], r.rndr.quote, typ)
	}

	tblTpl := `CREATE TABLE IF NOT EXISTS "%s" ( %s )`
	stmt := fmt.Sprintf(tblTpl, tblName, strings.Join(colExprs, ", "))
	lg.Debugf("returning SQL: %s", stmt)
	return stmt, nil
}

func (r *SQLite3Renderer) getNativeType(typ reflect.Type) string {

	const AffinityText = `TEXT`
	const AffinityNumeric = `NUMERIC`
	const AffinityInteger = `INTEGER`
	const AffinityReal = `REAL`
	const AffinityBlob = `BLOB`

	if typ == nil {
		return AffinityText
	}

	switch typ {
	case ast.TypeNullInt64:
		return AffinityInteger
	case ast.TypeNullFloat64:
		return AffinityReal
	case ast.TypeByteArray:
		return AffinityBlob
	}

	return AffinityText
}

func (r *SQLite3Renderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *SQLite3Renderer) FromTable(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *SQLite3Renderer) Join(fnJoin *ast.Join) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *SQLite3Renderer) Range(rr *ast.RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *SQLite3Renderer) SelectCols(cols []ast.ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type XLSXRenderer struct {
	rndr *baseRenderer
}

func (r *XLSXRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *XLSXRenderer) FromTable(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *XLSXRenderer) Join(fnJoin *ast.Join) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *XLSXRenderer) Range(rr *ast.RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *XLSXRenderer) SelectCols(cols []ast.ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type CSVRenderer struct {
	rndr *baseRenderer
}

func (r *CSVRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *CSVRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *CSVRenderer) SelectAll(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *CSVRenderer) FromTable(tblSel *ast.TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *CSVRenderer) Join(fnJoin *ast.Join) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *CSVRenderer) Range(rr *ast.RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *CSVRenderer) SelectCols(cols []ast.ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}
