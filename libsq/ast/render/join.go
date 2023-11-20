package render

import (
	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
)

func renderJoinType(jt jointype.Type) (string, error) {
	switch jt {
	case jointype.Inner:
		return "INNER JOIN", nil
	case jointype.Left:
		return "LEFT JOIN", nil
	case jointype.LeftOuter:
		return "LEFT OUTER JOIN", nil
	case jointype.Right:
		return "RIGHT JOIN", nil
	case jointype.RightOuter:
		return "RIGHT OUTER JOIN", nil
	case jointype.FullOuter:
		return "FULL OUTER JOIN", nil
	case jointype.Cross:
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
		allTbls[i+1] = joins[i].Table()
	}

	sql := "FROM "
	sql = sqlAppend(sql, leftTbl.Table().Render(enquote))
	if leftTbl.Alias() != "" {
		sql = sqlAppend(sql, "AS "+enquote(leftTbl.Alias()))
	}

	for i, join := range joins {
		var s string
		var err error
		jt := join.JoinType()
		if !lo.Contains(rc.Dialect.Joins, jt) {
			return "", errz.Errorf("driver {%s} does not support join type {%s}",
				rc.Dialect.Type, jt)
		}
		if s, err = renderJoinType(jt); err != nil {
			return "", err
		}

		tbl := join.Table()
		s = sqlAppend(s, tbl.Table().Render(enquote))
		if tbl.Alias() != "" {
			s = sqlAppend(s, "AS "+enquote(tbl.Alias()))
		}

		if expr := join.Predicate(); expr != nil {
			if !join.JoinType().HasPredicate() {
				return "", errz.Errorf("invalid join: {%s} does not accept a predicate: %s",
					join.JoinType(), join.Text())
			}

			s = sqlAppend(s, "ON")

			// Special handling for: .left_tbl | join(.right_tbl, .col)
			// This is rendered as:
			//  FROM left_tbl JOIN right_tbl ON left_tbl.col = right_tbl.col
			children := expr.Children()
			if len(children) == 1 {
				if colSel, ok := children[0].(*ast.ColSelectorNode); ok {
					// TODO: should be able to handle ast.TblColSelector also?
					colName := colSel.ColName()
					text := allTbls[i].TblAliasOrName().Render(enquote) + "." + enquote(colName)
					text += " = "
					text += allTbls[i+1].TblAliasOrName().Render(enquote) + "." + enquote(colName)
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
}
