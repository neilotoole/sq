package render

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func renderJoinType(jt ast.JoinType) (string, error) {
	switch jt {
	case ast.Join, ast.JoinInner:
		return "INNER JOIN", nil
	case ast.JoinLeft:
		return "LEFT JOIN", nil
	case ast.JoinLeftOuter:
		return "LEFT OUTER JOIN", nil
	case ast.JoinRight:
		return "RIGHT JOIN", nil
	case ast.JoinRightOuter:
		return "RIGHT OUTER JOIN", nil
	case ast.JoinFullOuter:
		return "FULL OUTER JOIN", nil
	case ast.JoinCross:
		return "CROSS JOIN", nil
	default:
		return "", errz.Errorf("unknown join type: %s", jt)
	}
}

func doJoin(rc *Context, leftTbl *ast.TblSelectorNode, joins []*ast.JoinNode) (string, error) {
	enquote := rc.Dialect.Enquote

	allTbls := make([]*ast.TblSelectorNode, len(joins)+1)
	allTbls[0] = leftTbl
	for i := range joins {
		allTbls[i+1] = joins[i].RightTbl()
	}

	sql := "FROM "
	sql = sqlAppend(sql, enquote(leftTbl.TblName()))
	if leftTbl.Alias() != "" {
		sql = sqlAppend(sql, enquote(leftTbl.Alias()))
	}

	for i, join := range joins {
		var s string
		var err error
		if s, err = renderJoinType(join.JoinType()); err != nil {
			return "", err
		}

		tbl := join.RightTbl()
		s = sqlAppend(s, enquote(tbl.TblName()))
		if tbl.Alias() != "" {
			s = sqlAppend(s, enquote(tbl.Alias()))
		}

		if expr := join.Constraint(); expr != nil {
			s = sqlAppend(s, "ON")

			// Special handling for: .left_tbl | join(.right_tbl, .col)
			// This is rendered as:
			//  FROM left_tbl JOIN right_tbl ON left_tbl.col = right_tbl.col
			children := expr.Children()
			if len(children) == 1 {
				if colSel, ok := children[0].(*ast.ColSelectorNode); ok {
					// TODO: should be able to handle ast.TblColSelector also?
					colName := colSel.ColName()
					text := enquote(allTbls[i].TblAliasOrName()) + "." + enquote(colName)
					text += " = "
					text += enquote(allTbls[i+1].TblAliasOrName()) + "." + enquote(colName)
					s = sqlAppend(s, text)
					sql = sqlAppend(sql, s)
					continue
				}
			}

			var text string
			if text, err = rc.Renderer.Expr(rc, expr); err != nil {
				return "", err
			}

			s = sqlAppend(s, text)
		}

		sql = sqlAppend(sql, s)
	}

	return sql, nil
	//
	//sql, err := renderJoinType(joinNode.JoinType())
	//if err != nil {
	//	return "", err
	//}
	//
	//rightTbl, err := renderSelectorNode(rc.Dialect, joinNode.RightTbl())
	//if err != nil {
	//	return "", err
	//}
	//
	//sql = sqlAppend(sql, rightTbl)
	//
	//constraintExpr := joinNode.Constraint()
	//
	//if constraintExpr == nil {
	//	return sql, nil
	//}
	//
	//constraint, err := rc.Renderer.Expr(rc, constraintExpr)
	//if err != nil {
	//	return "", err
	//}
	//
	//sql = sqlAppend(sql, "ON "+constraint)
	//return sql, nil
	//
	//
	//
	//if len(joinNode.Children()) == 0 {
	//	renderedJoinType = "NATURAL JOIN"
	//} else {
	//	joinExpr, ok := joinNode.Children()[0].(*ast.JoinConstraint)
	//	if !ok {
	//		return "", errz.Errorf("expected *FnJoinExpr but got %T", joinNode.Children()[0])
	//	}
	//
	//	leftOperand := ""
	//	operator := ""
	//	rightOperand := ""
	//
	//	if len(joinExpr.Children()) == 1 {
	//		// It's a single col selector
	//		colSel, ok := joinExpr.Children()[0].(*ast.ColSelectorNode)
	//		if !ok {
	//			return "", errz.Errorf("expected *ColSelectorNode but got %T", joinExpr.Children()[0])
	//		}
	//
	//		colVal, err := colSel.SelValue()
	//		if err != nil {
	//			return "", err
	//		}
	//
	//		leftTblVal := joinNode.LeftTbl().TblName()
	//		leftOperand = fmt.Sprintf(
	//			"%s.%s",
	//			enquote(leftTblVal),
	//			enquote(colVal),
	//		)
	//
	//		operator = "=="
	//
	//		rightTblVal := joinNode.RightTbl().TblName()
	//		rightOperand = fmt.Sprintf(
	//			"%s.%s",
	//			enquote(rightTblVal),
	//			enquote(colVal),
	//		)
	//	} else {
	//		var err error
	//		leftOperand, err = renderSelectorNode(rc.Dialect, joinExpr.Children()[0])
	//		if err != nil {
	//			return "", err
	//		}
	//
	//		operator = joinExpr.Children()[1].Text()
	//
	//		rightOperand, err = renderSelectorNode(rc.Dialect, joinExpr.Children()[2])
	//		if err != nil {
	//			return "", err
	//		}
	//	}
	//
	//	if operator == "==" {
	//		operator = "="
	//	}
	//
	//	onClause = fmt.Sprintf("ON %s %s %s", leftOperand, operator, rightOperand)
	//}
	//
	//sql := fmt.Sprintf(
	//	"FROM %s %s %s",
	//	enquote(joinNode.LeftTbl().TblName()),
	//	renderedJoinType,
	//	enquote(joinNode.RightTbl().TblName()),
	//)
	//sql = sqlAppend(sql, onClause)
	//
	//return sql, nil
}
