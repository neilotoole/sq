package ql

import (
	"fmt"

	"strings"

	"math"

	"reflect"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/driver"
)

var renderers = make(map[driver.Type]Renderer)

// Renderer renders driver-specific SQL fragments.
type Renderer interface {
	FromTable(tblSel *TblSelector) (string, error)
	SelectCols(cols []ColExpr) (string, error)
	SelectAll(tblSel *TblSelector) (string, error)
	Range(rr *RowRange) (string, error)
	Join(fnJoin *FnJoin) (string, error)
	CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error)
	CreateInsertStmt(tblName string, colNames []string) (string, error)
	Metadata(dbName string) (string, error)
	TableMetadata(dbName string, tblName string) (string, error)
}

func init() {
	renderers[driver.Type("mysql")] = &MySQLRenderer{rndr: &baseRenderer{quote: "`"}}
	renderers[driver.Type("postgres")] = &PostgresRenderer{rndr: &baseRenderer{quote: `"`}}
	renderers[driver.Type("sqlite3")] = &SQLite3Renderer{rndr: &baseRenderer{quote: `"`}}
	renderers[driver.Type("xlsx")] = &XLSXRenderer{rndr: &baseRenderer{quote: `"`}}
}

// RendererFor returns a Renderer for the given driver type.
func RendererFor(typ driver.Type) (Renderer, error) {

	r, ok := renderers[typ]
	if !ok {
		return nil, errorf("renderer for driver %q not found", typ)
	}

	return r, nil
}

type baseRenderer struct {
	// quote is the driver-specific quote rune, e.g. " or `
	quote string
}

func (r *baseRenderer) SelectAll(tblSel *TblSelector) (string, error) {
	sql := fmt.Sprintf("SELECT * FROM %v%s%v", r.quote, tblSel.SelValue(), r.quote)
	lg.Debugf("returning SQL fragment: %s", sql)
	return sql, nil
}
func (r *baseRenderer) FromTable(tblSel *TblSelector) (string, error) {

	tblName := tblSel.SelValue()
	if tblName == "" {
		return "", errorf("selector has empty table name: %q", tblSel.Text())
	}

	clause := fmt.Sprintf("FROM %v%s%v", r.quote, tblSel.SelValue(), r.quote)
	lg.Debugf("returning SQL fragment: %s", clause)
	return clause, nil
}

func (r *baseRenderer) Join(fnJoin *FnJoin) (string, error) {

	joinType := "INNER JOIN"

	onClause := ""

	if len(fnJoin.Children()) == 0 {
		joinType = "NATURAL JOIN"
	} else {
		joinExpr, ok := fnJoin.Children()[0].(*FnJoinExpr)
		if !ok {
			return "", errorf("expected *FnJoinExpr but got %T", fnJoin.Children()[0])
		}

		leftOperand := ""
		operator := ""
		rightOperand := ""

		if len(joinExpr.Children()) == 1 {
			// It's a single col selector
			colSel, ok := joinExpr.Children()[0].(*ColSelector)
			if !ok {
				return "", errorf("expected *ColSelector but got %T", joinExpr.Children()[0])
			}

			leftOperand = fmt.Sprintf("%s%s%s.%s%s%s", r.quote, fnJoin.leftTbl.SelValue(), r.quote, r.quote, colSel.SelValue(), r.quote)
			operator = "=="
			rightOperand = fmt.Sprintf("%s%s%s.%s%s%s", r.quote, fnJoin.rightTbl.SelValue(), r.quote, r.quote, colSel.SelValue(), r.quote)
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

	sql := fmt.Sprintf("FROM %s%s%s %s %s%s%s", r.quote, fnJoin.leftTbl.SelValue(), r.quote, joinType, r.quote, fnJoin.rightTbl.SelValue(), r.quote)
	if onClause != "" {
		sql = sql + " " + onClause
	}

	lg.Debugf("returning JOIN fragment: %s", sql)
	return sql, nil
}
func (r *baseRenderer) Range(rr *RowRange) (string, error) {

	if rr == nil {
		return "", nil
	}

	if rr.limit < 0 && rr.offset < 0 {
		return "", nil
	}

	limit := ""
	offset := ""
	if rr.limit > -1 {
		limit = fmt.Sprintf(" LIMIT %d", rr.limit)
	}
	if rr.offset > -1 {
		offset = fmt.Sprintf(" OFFSET %d", rr.offset)

		if rr.limit == -1 {
			// MySQL requires a LIMIT if OFFSET is used. Therefore
			// we make the LIMIT a very large number
			limit = fmt.Sprintf(" LIMIT %d", math.MaxInt64)
		}
	}

	sql := limit + offset
	lg.Debugf("returning SQL fragment: %s", sql)

	return sql, nil
}

func (r *baseRenderer) SelectCols(cols []ColExpr) (string, error) {

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
			vals[i] = fmt.Sprintf("`%s`", colText)
			continue
		}

		// the expr contains a period, so it's likely scoped, e.g. "user.uid"
		parts := strings.Split(colText, ".")
		if len(parts) != 2 {
			return "", errorf("expected scoped col expr %q to have 2 parts, but got: %v", col, parts)
		}

		vals[i] = fmt.Sprintf("`%s`.`%s`", parts[0], parts[1])
	}

	text := "SELECT " + strings.Join(vals, ", ")
	return text, nil
}

type MySQLRenderer struct {
	rndr *baseRenderer
}

func (r *MySQLRenderer) Metadata(dbName string) (string, error) {

	tpl := "SELECT TABLE_SCHEMA AS `schema_name`,TABLE_NAME AS `table_name`, TABLE_COMMENT AS `table_comment`, (DATA_LENGTH + INDEX_LENGTH) AS `table_size` FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME ASC"
	//sql := fmt.Sprintf(tpl, dbName)

	sql := tpl
	//sql := fmt.Sprintf(tpl, r.rndr.quote, dbName, r.rndr.quote)
	lg.Debugf("returning SQL: %s", sql)

	//SELECT table_schema, table_name,GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR ', ') AS `col_names`, GROUP_CONCAT(column_type ORDER BY ordinal_position SEPARATOR ', ') AS `col_type`
	//FROM information_schema.columns
	//WHERE table_schema = DATABASE()
	//GROUP BY table_name
	//ORDER BY table_name

	return sql, nil
}

func (r *MySQLRenderer) TableMetadata(dbName string, tblName string) (string, error) {
	tpl := "SELECT column_name, data_type, column_type, ordinal_position, is_nullable, column_key, column_comment, extra, (SELECT COUNT(*) FROM `%s`) AS row_count FROM information_schema.columns cols WHERE cols.TABLE_SCHEMA = '%s' AND cols.TABLE_NAME = '%s' ORDER BY cols.ordinal_position ASC"
	sql := fmt.Sprintf(tpl, tblName, dbName, tblName)
	lg.Debugf("returning SQL: %s", sql)
	return sql, nil
}

func (r *MySQLRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *MySQLRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *MySQLRenderer) SelectAll(tblSel *TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *MySQLRenderer) FromTable(tblSel *TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *MySQLRenderer) Join(fnJoin *FnJoin) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *MySQLRenderer) Range(rr *RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *MySQLRenderer) SelectCols(cols []ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type PostgresRenderer struct {
	rndr *baseRenderer
}

func (r *PostgresRenderer) Metadata(dbName string) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) TableMetadata(dbName string, tblName string) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *PostgresRenderer) SelectAll(tblSel *TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *PostgresRenderer) FromTable(tblSel *TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *PostgresRenderer) Join(fnJoin *FnJoin) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *PostgresRenderer) Range(rr *RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *PostgresRenderer) SelectCols(cols []ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type SQLite3Renderer struct {
	rndr *baseRenderer
}

func (r *SQLite3Renderer) Metadata(dbName string) (string, error) {
	return "", errorf("not implemented")
}

func (r *SQLite3Renderer) TableMetadata(dbName string, tblName string) (string, error) {
	return "", errorf("not implemented")
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

	//return "", errorf("not implemented")
}

func (r *SQLite3Renderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {

	lg.Debugf("table name: %s | col names: %v", tblName, colNames)
	colExprs := make([]string, len(colNames))
	for i := 0; i < len(colNames); i++ {
		//colTypes[i] = "TEXT"
		typ := r.getNativeType(colTypes[i])
		colExprs[i] = fmt.Sprintf(`%s%s%s %s`, r.rndr.quote, colNames[i], r.rndr.quote, typ)
	}

	tblTpl := `CREATE TABLE IF NOT EXISTS "%s" ( %s )`
	stmt := fmt.Sprintf(tblTpl, tblName, strings.Join(colExprs, ", "))
	lg.Debugf("returning SQL: %s", stmt)
	return stmt, nil
}

func (r *SQLite3Renderer) getNativeType(typ reflect.Type) string {

	//lg.Debugf("%v", TypeNullInt64)
	//lg.Debugf("%v", TypeNullFloat64)
	//lg.Debugf("%v", TypeNullString)
	//lg.Debugf("%v", TypeByteArray)

	const AffinityText = `TEXT`
	const AffinityNumeric = `NUMERIC`
	const AffinityInteger = `INTEGER`
	const AffinityReal = `REAL`
	const AffinityBlob = `BLOB`

	if typ == nil {
		return AffinityText
	}

	switch typ {
	case TypeNullInt64:
		return AffinityInteger
	case TypeNullFloat64:
		return AffinityReal
	case TypeByteArray:
		return AffinityBlob
	}

	return AffinityText
}

func (r *SQLite3Renderer) SelectAll(tblSel *TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *SQLite3Renderer) FromTable(tblSel *TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *SQLite3Renderer) Join(fnJoin *FnJoin) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *SQLite3Renderer) Range(rr *RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *SQLite3Renderer) SelectCols(cols []ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}

type XLSXRenderer struct {
	rndr *baseRenderer
}

func (r *XLSXRenderer) Metadata(dbName string) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) TableMetadata(dbName string, tblName string) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) CreateTable(tblName string, colNames []string, colTypes []reflect.Type) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) CreateInsertStmt(tblName string, colNames []string) (string, error) {
	return "", errorf("not implemented")
}

func (r *XLSXRenderer) SelectAll(tblSel *TblSelector) (string, error) {
	return r.rndr.SelectAll(tblSel)
}

func (r *XLSXRenderer) FromTable(tblSel *TblSelector) (string, error) {
	return r.rndr.FromTable(tblSel)
}

func (r *XLSXRenderer) Join(fnJoin *FnJoin) (string, error) {
	return r.rndr.Join(fnJoin)
}
func (r *XLSXRenderer) Range(rr *RowRange) (string, error) {
	return r.rndr.Range(rr)
}

func (r *XLSXRenderer) SelectCols(cols []ColExpr) (string, error) {
	return r.rndr.SelectCols(cols)
}
