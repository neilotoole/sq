package render

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doJoin(rc *Context, fnJoin *ast.JoinNode) (string, error) {
	// FIXME: switch to using dialect.Dialect.Enquote.

	quote := string(rc.Dialect.IdentQuote)

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
				quote,
				leftTblVal,
				quote,
				quote,
				colVal,
				quote,
			)

			operator = "=="

			rightTblVal := fnJoin.RightTbl().TblName()
			rightOperand = fmt.Sprintf(
				"%s%s%s.%s%s%s",
				quote,
				rightTblVal,
				quote,
				quote,
				colVal,
				quote,
			)
		} else {
			var err error
			leftOperand, err = renderSelectorNode(quote, joinExpr.Children()[0])
			if err != nil {
				return "", err
			}

			operator = joinExpr.Children()[1].Text()

			rightOperand, err = renderSelectorNode(quote, joinExpr.Children()[2])
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
		quote,
		fnJoin.LeftTbl().TblName(),
		quote,
		joinType,
		quote,
		fnJoin.RightTbl().TblName(),
		quote,
	)
	sql = sqlAppend(sql, onClause)

	return sql, nil
}
