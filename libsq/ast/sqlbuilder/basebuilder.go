package sqlbuilder

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

const singleQuote = '\''

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

var _ FragmentBuilder = (*BaseFragmentBuilder)(nil)

// BaseFragmentBuilder is a default implementation of sqlbuilder.FragmentBuilder.
type BaseFragmentBuilder struct {
	Log lg.Log
	// Quote is the driver-specific quote rune, e.g. " or `
	Quote    string
	ColQuote string
	Ops      map[string]string
}

// GroupBy implements FragmentBuilder.
func (fb *BaseFragmentBuilder) GroupBy(gb *ast.GroupByNode) (string, error) {
	if gb == nil {
		return "", nil
	}

	clause := "GROUP BY "
	children := gb.Children()
	for i := 0; i < len(children); i++ {
		if i > 0 {
			clause += ", "
		}

		// FIXME: really should check for other types
		s, err := renderSelectorNode(fb.Quote, children[i])
		if err != nil {
			return "", err
		}

		clause += s
	}

	return clause, nil
}

// OrderBy implements FragmentBuilder.
func (fb *BaseFragmentBuilder) OrderBy(ob *ast.OrderByNode) (string, error) {
	if ob == nil {
		return "", nil
	}

	terms := ob.Terms()
	if len(terms) == 0 {
		return "", errz.Errorf("%T has no ordering terms: %s", ob, ob)
	}

	clause := "ORDER BY "
	for i := 0; i < len(terms); i++ {
		if i > 0 {
			clause += ", "
		}

		sel, err := renderSelectorNode(fb.Quote, terms[i].Selector())
		if err != nil {
			return "", err
		}

		clause += sel
		switch terms[i].Direction() { //nolint:exhaustive
		case ast.OrderByDirectionAsc:
			clause += " ASC"
		case ast.OrderByDirectionDesc:
			clause += " DESC"
		default:
		}
	}

	return clause, nil
}

// Operator implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Operator(op *ast.OperatorNode) (string, error) {
	if val, ok := fb.Ops[op.Text()]; ok {
		return val, nil
	}

	return op.Text(), nil
}

// Where implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Where(where *ast.WhereNode) (string, error) {
	sql, err := fb.Expr(where.Expr())
	if err != nil {
		return "", err
	}

	sql = "WHERE " + sql
	return sql, nil
}

// Expr implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Expr(expr *ast.ExprNode) (string, error) {
	var sql string

	for _, child := range expr.Children() {
		switch child := child.(type) {
		case *ast.TblColSelectorNode, *ast.ColSelectorNode:
			val, err := renderSelectorNode(fb.ColQuote, child)
			if err != nil {
				return "", err
			}
			sql = val
		case *ast.OperatorNode:
			val, err := fb.Operator(child)
			if err != nil {
				return "", err
			}
			sql = sql + " " + val
		case *ast.ExprNode:
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
func (fb *BaseFragmentBuilder) SelectAll(tblSel *ast.TblSelectorNode) (string, error) {
	sql := fmt.Sprintf("SELECT * FROM %v%s%v", fb.Quote, tblSel.TblName(), fb.Quote)
	return sql, nil
}

// Function implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Function(fn *ast.FuncNode) (string, error) {
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

	buf.WriteString(strings.ToUpper(fn.FuncName()))
	buf.WriteRune('(')
	for i, child := range children {
		if i > 0 {
			buf.WriteString(", ")
		}

		switch node := child.(type) {
		case *ast.ColSelectorNode, *ast.TblColSelectorNode, *ast.TblSelectorNode:
			s, err := renderSelectorNode(fb.Quote, node)
			if err != nil {
				return "", err
			}
			buf.WriteString(s)
		case *ast.OperatorNode:
			buf.WriteString(node.Text())
		case *ast.LiteralNode:
			// TODO: This is all a bit of a mess. We probably need to
			// move to using bound parameters instead of inlining
			// literal values.
			val, wasQuoted, err := unquoteLiteral(node.Text())
			if err != nil {
				return "", err
			}

			if wasQuoted {
				// The literal had quotes, so it's a regular string.
				buf.WriteRune(singleQuote)
				buf.WriteString(escapeLiteralString(val))
				buf.WriteRune(singleQuote)
			} else {
				buf.WriteString(val)
			}
		default:
			return "", errz.Errorf("unknown AST child node %T: %s", node, node)
		}
	}

	buf.WriteRune(')')
	sql := buf.String()
	return sql, nil
}

// escapeLiteralString escapes the single quotes in s.
func escapeLiteralString(s string) string {
	if !strings.ContainsRune(s, singleQuote) {
		return s
	}

	sb := strings.Builder{}
	for _, r := range s {
		if r == singleQuote {
			_, _ = sb.WriteRune(singleQuote)
			_, _ = sb.WriteRune(singleQuote)
			continue
		}

		_, _ = sb.WriteRune(r)
	}

	return sb.String()
}

// unquoteLiteral returns true if s is a "quoted" string, and also returns
// the value with the quotes stripped. An error is returned if the string
// is malformed.
func unquoteLiteral(s string) (val string, ok bool, err error) {
	hasPrefix := strings.HasPrefix(s, `"`)
	hasSuffix := strings.HasSuffix(s, `"`)

	if hasPrefix && hasSuffix {
		val = strings.TrimPrefix(s, `"`)
		val = strings.TrimSuffix(val, `"`)
		return val, true, nil
	}

	if hasPrefix != hasSuffix {
		return "", false, errz.Errorf("malformed literal: %s", s)
	}

	return s, false, nil
}

// FromTable implements FragmentBuilder.
func (fb *BaseFragmentBuilder) FromTable(tblSel *ast.TblSelectorNode) (string, error) {
	tblName, _ := tblSel.SelValue()
	if tblName == "" {
		return "", errz.Errorf("selector has empty table name: %q", tblSel.Text())
	}

	clause := fmt.Sprintf("FROM %v%s%v", fb.Quote, tblSel.TblName(), fb.Quote)
	return clause, nil
}

// Join implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Join(fnJoin *ast.JoinNode) (string, error) {
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
			colSel, ok := joinExpr.Children()[0].(*ast.ColSelectorNode)
			if !ok {
				return "", errz.Errorf("expected *ColSelectorNode but got %T", joinExpr.Children()[0])
			}

			colVal, err := colSel.SelValue()
			if err != nil {
				return "", err
			}

			leftTblVal := fnJoin.LeftTbl().TblName()
			leftOperand = fmt.Sprintf(
				"%s%s%s.%s%s%s",
				fb.Quote,
				leftTblVal,
				fb.Quote,
				fb.Quote,
				colVal,
				fb.Quote,
			)

			operator = "=="

			rightTblVal := fnJoin.RightTbl().TblName()
			rightOperand = fmt.Sprintf(
				"%s%s%s.%s%s%s",
				fb.Quote,
				rightTblVal,
				fb.Quote,
				fb.Quote,
				colVal,
				fb.Quote,
			)
		} else {
			var err error
			leftOperand, err = renderSelectorNode(fb.Quote, joinExpr.Children()[0])
			if err != nil {
				return "", err
			}

			operator = joinExpr.Children()[1].Text()

			rightOperand, err = renderSelectorNode(fb.Quote, joinExpr.Children()[2])
			if err != nil {
				return "", err
			}
		}

		if operator == "==" {
			operator = "="
		}

		onClause = fmt.Sprintf("ON %s %s %s", leftOperand, operator, rightOperand)
	}

	sql := fmt.Sprintf(
		"FROM %s%s%s %s %s%s%s",
		fb.Quote,
		fnJoin.LeftTbl().TblName(),
		fb.Quote,
		joinType,
		fb.Quote,
		fnJoin.RightTbl().TblName(),
		fb.Quote,
	)
	sql = sqlAppend(sql, onClause)

	return sql, nil
}

// renderSelectorNode renders a selector such as ".actor.first_name"
// or ".last_name".
func renderSelectorNode(quote string, node ast.Node) (string, error) {
	switch node := node.(type) {
	case *ast.ColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s.%s%s%s",
			quote,
			node.TblName(),
			quote,
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.TblName(),
			quote,
		), nil

	default:
		return "", errz.Errorf(
			"expected selector node type, but got %T: %s",
			node,
			node.Text(),
		)
	}
}

// sqlAppend is a convenience function for building the SQL string.
// The main purpose is to ensure that there's always a consistent amount
// of whitespace. Thus, if existing has a space suffix and add has a
// space prefix, the returned string will only have one space. If add
// is the empty string or just whitespace, this function simply
// returns existing.
func sqlAppend(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}

	existing = strings.TrimSpace(existing)
	return existing + " " + add
}

// quoteTableOrColSelector returns a quote table, col, or table/col
// selector for use in a SQL statement. For example:
//
//	.table     -->  "table"
//	.col       -->  "col"
//	.table.col -->  "table"."col"
//
// Thus, the selector must have exactly one or two periods.
func quoteTableOrColSelector(quote, selector string) (string, error) {
	if len(selector) < 2 || selector[0] != '.' {
		return "", errz.Errorf("invalid selector: %s", selector)
	}

	parts := strings.Split(selector[1:], ".")
	switch len(parts) {
	case 1:
		return quote + parts[0] + quote, nil
	case 2:
		return quote + parts[0] + quote + "." + quote + parts[1] + quote, nil
	default:
		return "", errz.Errorf("invalid selector: %s", selector)
	}
}

// Range implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Range(rr *ast.RowRangeNode) (string, error) {
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
func (fb *BaseFragmentBuilder) SelectCols(cols []ast.ResultColumn) (string, error) {
	if len(cols) == 0 {
		return "SELECT *", nil
	}

	vals := make([]string, len(cols))
	for i, col := range cols {
		// aliasFrag holds the "AS alias" fragment (if applicable).
		// For example: "@sakila | .actor | .first_name:given_name" becomes
		// "SELECT first_name AS given_name FROM actor".
		var aliasFrag string
		if col.Alias() != "" {
			aliasFrag = fmt.Sprintf(" AS %s%s%s", fb.Quote, col.Alias(), fb.Quote)
		}

		switch col := col.(type) {
		case *ast.ColSelectorNode:
			vals[i] = fmt.Sprintf("%s%s%s", fb.Quote, col.ColName(), fb.Quote)
		case *ast.TblColSelectorNode:
			vals[i] = fmt.Sprintf("%s%s%s.%s%s%s", fb.Quote, col.TblName(), fb.Quote, fb.Quote, col.ColName(), fb.Quote)
		case *ast.FuncNode:
			// it's a function
			var err error
			if vals[i], err = fb.Function(col); err != nil {
				return "", err
			}
		default:
			// FIXME: We should be exhaustively checking the cases.
			// it's probably an expression
			vals[i] = col.Text() // for now, we just return the raw text
		}

		vals[i] += aliasFrag
	}

	text := "SELECT " + strings.Join(vals, ", ")
	return text, nil
}

var _ QueryBuilder = (*BaseQueryBuilder)(nil)

// BaseQueryBuilder is a default implementation
// of sqlbuilder.QueryBuilder.
type BaseQueryBuilder struct {
	SelectClause  string
	FromClause    string
	WhereClause   string
	RangeClause   string
	OrderByClause string
	GroupByClause string
}

// SetGroupBy implements QueryBuilder.
func (qb *BaseQueryBuilder) SetGroupBy(gb string) {
	qb.GroupByClause = gb
}

// SetOrderBy implements QueryBuilder.
func (qb *BaseQueryBuilder) SetOrderBy(ob string) {
	qb.OrderByClause = ob
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

	if qb.GroupByClause != "" {
		buf.WriteRune(' ')
		buf.WriteString(qb.GroupByClause)
	}

	if qb.RangeClause != "" {
		buf.WriteRune(' ')
		buf.WriteString(qb.RangeClause)
	}

	return buf.String(), nil
}
