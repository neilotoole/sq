package sqlbuilder

import (
	"fmt"
	"math"
	"strings"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

const (
	singleQuote = '\''
	sp          = ' '
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

var _ FragmentBuilder = (*BaseFragmentBuilder)(nil)

// BaseFragmentBuilder is a default implementation of sqlbuilder.FragmentBuilder.
type BaseFragmentBuilder struct {
	Log *slog.Logger
	// Quote is the driver-specific quote rune, e.g. " or `
	Quote string

	// QuoteFn quotes an identifier.
	QuoteFn func(string) string
	Ops     map[string]string
}

// Distinct implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Distinct(n *ast.UniqueNode) (string, error) {
	if n == nil {
		return "", nil
	}
	return "DISTINCT", nil
}

// GroupBy implements FragmentBuilder.
func (fb *BaseFragmentBuilder) GroupBy(gb *ast.GroupByNode) (string, error) {
	if gb == nil {
		return "", nil
	}

	var (
		term string
		err  error
		sb   strings.Builder
	)

	sb.WriteString("GROUP BY ")
	for i, child := range gb.Children() {
		if i > 0 {
			sb.WriteString(", ")
		}

		switch child := child.(type) {
		case *ast.FuncNode:
			if term, err = fb.Function(child); err != nil {
				return "", err
			}
		case ast.Selector:
			if term, err = renderSelectorNode(fb.Quote, child); err != nil {
				return "", err
			}
		default:
			// Should never happen
			return "", errz.Errorf("invalid child type: %T: %s", child, child)
		}

		sb.WriteString(term)
	}

	return sb.String(), nil
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
	if op == nil {
		return "", nil
	}

	if val, ok := fb.Ops[op.Text()]; ok {
		return val, nil
	}

	return op.Text(), nil
}

// Where implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Where(where *ast.WhereNode) (string, error) {
	if where == nil {
		return "", nil
	}
	sql, err := fb.Expr(where.Expr())
	if err != nil {
		return "", err
	}

	sql = "WHERE " + sql
	return sql, nil
}

// Expr implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Expr(expr *ast.ExprNode) (string, error) {
	if expr == nil {
		return "", nil
	}
	var sb strings.Builder

	for _, child := range expr.Children() {
		switch child := child.(type) {
		case *ast.TblColSelectorNode, *ast.ColSelectorNode:
			val, err := renderSelectorNode(fb.Quote, child)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
		case *ast.OperatorNode:
			val, err := fb.Operator(child)
			if err != nil {
				return "", err
			}

			sb.WriteRune(sp)
			sb.WriteString(val)
		case *ast.ExprNode:
			val, err := fb.Expr(child)
			if err != nil {
				return "", err
			}
			sb.WriteRune(sp)
			sb.WriteString(val)
		default:
			sb.WriteRune(sp)
			sb.WriteString(child.Text())
		}
	}

	return sb.String(), nil
}

// Function implements FragmentBuilder.
func (fb *BaseFragmentBuilder) Function(fn *ast.FuncNode) (string, error) {
	sb := strings.Builder{}
	fnName := strings.ToLower(fn.FuncName())
	children := fn.Children()

	if len(children) == 0 {
		sb.WriteString(fnName)
		sb.WriteRune('(')

		if fnName == "count" {
			// Special handling for the count function, because COUNT()
			// isn't valid, but COUNT(*) is.
			sb.WriteRune('*')
		}

		sb.WriteRune(')')
		return sb.String(), nil
	}

	// Special handling for "count_unique(.col)" function. We translate
	// it to "SELECT count(DISTINCT col)".
	if fnName == "count_unique" {
		sb.WriteString("count(DISTINCT ")
	} else {
		sb.WriteString(fnName)
		sb.WriteRune('(')
	}

	for i, child := range children {
		if i > 0 {
			sb.WriteString(", ")
		}

		switch node := child.(type) {
		case *ast.ColSelectorNode, *ast.TblColSelectorNode, *ast.TblSelectorNode:
			s, err := renderSelectorNode(fb.Quote, node)
			if err != nil {
				return "", err
			}
			sb.WriteString(s)
		case *ast.OperatorNode:
			sb.WriteString(node.Text())
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
				// FIXME: replace with stringz.SingleQuote
				sb.WriteRune(singleQuote)
				sb.WriteString(escapeLiteral(val))
				sb.WriteRune(singleQuote)
			} else {
				sb.WriteString(val)
			}
		default:
			return "", errz.Errorf("unknown AST child node %T: %s", node, node)
		}
	}

	sb.WriteRune(')')
	sql := sb.String()
	return sql, nil
}

// FromTable implements FragmentBuilder.
func (fb *BaseFragmentBuilder) FromTable(tblSel *ast.TblSelectorNode) (string, error) {
	tblName, _ := tblSel.SelValue()
	if tblName == "" {
		return "", errz.Errorf("selector has empty table name: {%s}", tblSel.Text())
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
		return "*", nil
	}

	vals := make([]string, len(cols))
	for i, col := range cols {
		// aliasFrag holds the "AS alias" fragment (if applicable).
		// For example: "@sakila | .actor | .first_name:given_name" becomes
		// "SELECT first_name AS given_name FROM actor".
		var aliasFrag string
		if col.Alias() != "" {
			aliasFrag = " AS " + fb.QuoteFn(col.Alias())
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
			// Here, it's probably an ExprNode?
			vals[i] = col.Text() // for now, we just return the raw text
		}

		vals[i] += aliasFrag
	}

	text := strings.Join(vals, ", ")
	return text, nil
}

var _ QueryBuilder = (*BaseQueryBuilder)(nil)

// BaseQueryBuilder is a default implementation
// of sqlbuilder.QueryBuilder.
type BaseQueryBuilder struct {
	Distinct string
	Columns  string
	From     string
	Where    string
	GroupBy  string
	OrderBy  string
	Range    string
}

// SetDistinct implements QueryBuilder.
func (qb *BaseQueryBuilder) SetDistinct(d string) {
	qb.Distinct = d
}

// SetGroupBy implements QueryBuilder.
func (qb *BaseQueryBuilder) SetGroupBy(gb string) {
	qb.GroupBy = gb
}

// SetOrderBy implements QueryBuilder.
func (qb *BaseQueryBuilder) SetOrderBy(ob string) {
	qb.OrderBy = ob
}

// SetColumns implements QueryBuilder.
func (qb *BaseQueryBuilder) SetColumns(cols string) {
	qb.Columns = cols
}

// SetFrom implements QueryBuilder.
func (qb *BaseQueryBuilder) SetFrom(from string) {
	qb.From = from
}

// SetWhere implements QueryBuilder.
func (qb *BaseQueryBuilder) SetWhere(where string) {
	qb.Where = where
}

// SetRange implements QueryBuilder.
func (qb *BaseQueryBuilder) SetRange(rng string) {
	qb.Range = rng
}

// Render implements QueryBuilder.
func (qb *BaseQueryBuilder) Render() (string, error) {
	sb := strings.Builder{}

	sb.WriteString("SELECT")

	if qb.Distinct != "" {
		sb.WriteRune(sp)
		sb.WriteString(qb.Distinct)
	}

	sb.WriteRune(sp)
	sb.WriteString(qb.Columns)
	sb.WriteRune(sp)
	sb.WriteString(qb.From)

	if qb.Where != "" {
		sb.WriteRune(sp)
		sb.WriteString(qb.Where)
	}

	if qb.OrderBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(qb.OrderBy)
	}

	if qb.GroupBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(qb.GroupBy)
	}

	if qb.Range != "" {
		sb.WriteRune(sp)
		sb.WriteString(qb.Range)
	}

	return sb.String(), nil
}

// escapeLiteral escapes the single quotes in s.
//
//	jessie's girl  -->  jessie''s girl
func escapeLiteral(s string) string {
	sb := strings.Builder{}
	for _, r := range s {
		if r == singleQuote {
			_, _ = sb.WriteRune(singleQuote)
		}

		_, _ = sb.WriteRune(r)
	}

	return sb.String()
}

// unquoteLiteral returns true if s is a double-quoted string, and also returns
// the value with the quotes stripped. An error is returned if the string
// is malformed.
//
// REVISIT: why not use strconv.Unquote or such?
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
