package sqlbuilder

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/errz"
)

// baseOps is a map of SLQ operator (e.g. "==" or "!=") to its default SQL rendering.
var baseOps = map[string]string{
	`==`: `=`,
}

// BaseOps returns a default map of SLQ operator (e.g. "==" or "!=") to its default SQL rendering.
// The returned map is a copy and can be safely modified by the caller.
func BaseOps() map[string]string {
	ops := make(map[string]string, len(baseOps))
	for k, v := range baseOps {
		ops[k] = v
	}
	return ops
}

// BaseFragmentBuilder is a default implementation of sqlbuilder.FragmentBuilder.
type BaseFragmentBuilder struct {
	Log lg.Log
	// Quote is the driver-specific quote rune, e.g. " or `
	Quote    string
	ColQuote string
	Ops      map[string]string
}

// Operator implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Operator(op *ast.Operator) (string, error) {
	if val, ok := fb.Ops[op.Text()]; ok {
		return val, nil
	}

	return op.Text(), nil
}

// Where implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Where(where *ast.Where) (string, error) {
	sql, err := fb.Expr(where.Expr())
	if err != nil {
		return "", err
	}

	sql = "WHERE " + sql
	return sql, nil
}

// Expr implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Expr(expr *ast.Expr) (string, error) {
	var sql string

	for _, child := range expr.Children() {
		switch child := child.(type) {
		case *ast.Selector:
			val := child.SelValue()
			parts := strings.Split(val, ".")
			identifier := fb.ColQuote + strings.Join(parts, fb.ColQuote+"."+fb.ColQuote) + fb.ColQuote
			sql = sql + " " + identifier
		case *ast.Operator:
			val, err := fb.Operator(child)
			if err != nil {
				return "", err
			}
			sql = sql + " " + val
		case *ast.Expr:
			val, err := fb.Expr(child)
			if err != nil {
				return "", err
			}
			sql = sql + " " + val

		default:
			sql = sql + " " + child.Text()
		}
	}

	return sql, nil
}

// SelectAll implements FragmentBuilder.
func (fb *BaseFragmentBuilder) SelectAll(tblSel *ast.TblSelector) (string, error) {
	sql := fmt.Sprintf("SELECT * FROM %v%s%v", fb.Quote, tblSel.SelValue(), fb.Quote)
	return sql, nil
}

// Function implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Function(fn *ast.Func) (string, error) {

	buf := &bytes.Buffer{}
	children := fn.Children()

	if len(children) == 0 {
		// no children, let's just grab the direct text

		// HACK: this stuff basically doesn't work at all...
		//  but for COUNT(), here's a quick hack to make it work on some DBs
		if fn.Context().GetText() == "count()" {
			buf.WriteString("COUNT(*)")
		} else {
			buf.WriteString(fn.Context().GetText())
		}

		return buf.String(), nil
	}

	buf.WriteString(fn.FuncName())
	buf.WriteRune('(')
	for i, child := range children {
		if i > 0 {
			buf.WriteString(", ")
		}

		switch child := child.(type) {
		case *ast.ColSelector:
			buf.WriteString(child.SelValue())
		default:
			fb.Log.Debugf("unknown AST child node type %T", child)
		}
	}

	buf.WriteRune(')')
	sql := buf.String()
	return sql, nil
}

// FromTable implements FragmentBuilder.
func (fb *BaseFragmentBuilder) FromTable(tblSel *ast.TblSelector) (string, error) {
	tblName := tblSel.SelValue()
	if tblName == "" {
		return "", errz.Errorf("selector has empty table name: %q", tblSel.Text())
	}

	clause := fmt.Sprintf("FROM %v%s%v", fb.Quote, tblSel.SelValue(), fb.Quote)
	return clause, nil
}

// Join implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Join(fnJoin *ast.Join) (string, error) {
	joinType := "INNER JOIN"
	onClause := ""

	if len(fnJoin.Children()) == 0 {
		joinType = "NATURAL JOIN"
	} else {
		joinExpr, ok := fnJoin.Children()[0].(*ast.JoinConstraint)
		if !ok {
			return "", errz.Errorf("expected *FnJoinExpr but got %T", fnJoin.Children()[0])
		}

		leftOperand := ""
		operator := ""
		rightOperand := ""

		if len(joinExpr.Children()) == 1 {
			// It's a single col selector
			colSel, ok := joinExpr.Children()[0].(*ast.ColSelector)
			if !ok {
				return "", errz.Errorf("expected *ColSelector but got %T", joinExpr.Children()[0])
			}

			leftOperand = fmt.Sprintf("%s%s%s.%s%s%s", fb.Quote, fnJoin.LeftTbl().SelValue(), fb.Quote, fb.Quote, colSel.SelValue(), fb.Quote)
			operator = "=="
			rightOperand = fmt.Sprintf("%s%s%s.%s%s%s", fb.Quote, fnJoin.RightTbl().SelValue(), fb.Quote, fb.Quote, colSel.SelValue(), fb.Quote)
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

	sql := fmt.Sprintf("FROM %s%s%s %s %s%s%s", fb.Quote, fnJoin.LeftTbl().SelValue(), fb.Quote, joinType, fb.Quote, fnJoin.RightTbl().SelValue(), fb.Quote)
	if onClause != "" {
		sql = sql + " " + onClause
	}

	return sql, nil
}

// Range implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Range(rr *ast.RowRange) (string, error) {
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

	return sql, nil
}

// SelectCols implements FragmentBuilder.
func (fb *BaseFragmentBuilder) SelectCols(cols []ast.ColExpr) (string, error) {
	if len(cols) == 0 {
		return "SELECT *", nil
	}

	vals := make([]string, len(cols))

	for i, col := range cols {
		colText, err := col.ColExpr()
		if err != nil {
			return "", errz.Errorf("unable to extract col expr from %q: %v", col, err)
		}

		fn, ok := col.(*ast.Func)
		if ok {
			// it's a function
			vals[i], err = fb.Function(fn)
			if err != nil {
				return "", err
			}
			continue
		}

		if !col.IsColName() {
			// it's a function or expression
			vals[i] = colText // for now, we just return the raw text
			continue
		}

		// it's a column name, e.g. "uid" or "user.uid"
		if !strings.ContainsRune(colText, '.') {
			// it's a regular (non-scoped) col name, e.g. "uid"
			vals[i] = fmt.Sprintf("%s%s%s", fb.Quote, colText, fb.Quote)
			continue
		}

		// the expr contains a period, so it's likely scoped, e.g. "user.uid"
		parts := strings.Split(colText, ".")
		if len(parts) != 2 {
			return "", errz.Errorf("expected scoped col expr %q to have 2 parts, but got: %v", col, parts)
		}

		vals[i] = fmt.Sprintf("%s%s%s.%s%s%s", fb.Quote, parts[0], fb.Quote, fb.Quote, parts[1], fb.Quote)
	}

	text := "SELECT " + strings.Join(vals, ", ")
	return text, nil
}

// BaseQueryBuilder is a default implementation
// of sqlbuilder.QueryBuilder.
type BaseQueryBuilder struct {
	SelectClause  string
	FromClause    string
	WhereClause   string
	RangeClause   string
	OrderByClause string
}

// SetSelect implements QueryBuilder.
func (qb *BaseQueryBuilder) SetSelect(cols string) {
	qb.SelectClause = cols
}

// SetFrom implements QueryBuilder.
func (qb *BaseQueryBuilder) SetFrom(from string) {
	qb.FromClause = from
}

// SetWhere implements QueryBuilder.
func (qb *BaseQueryBuilder) SetWhere(where string) {
	qb.WhereClause = where
}

// SetRange implements QueryBuilder.
func (qb *BaseQueryBuilder) SetRange(rng string) {
	qb.RangeClause = rng
}

// SQL implements QueryBuilder.
func (qb *BaseQueryBuilder) SQL() (string, error) {
	buf := &bytes.Buffer{}

	buf.WriteString(qb.SelectClause)
	buf.WriteRune(' ')
	buf.WriteString(qb.FromClause)

	if qb.WhereClause != "" {
		buf.WriteRune(' ')
		buf.WriteString(qb.WhereClause)
	}

	if qb.OrderByClause != "" {
		buf.WriteRune(' ')
		buf.WriteString(qb.OrderByClause)
	}

	if qb.RangeClause != "" {
		buf.WriteRune(' ')
		buf.WriteString(qb.RangeClause)
	}

	return buf.String(), nil
}
