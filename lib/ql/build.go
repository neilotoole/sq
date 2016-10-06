package ql

import (
	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/ql/parser"
)

func BuildAST(query parser.IQueryContext) (*AST, error) {

	q, ok := query.(*parser.QueryContext)
	if !ok {
		return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	}

	v := &ParseTreeVisitor{}
	//v.AddListener(LogNodeVisit)
	q.Accept(v)
	if v.Err != nil {
		return nil, v.Err
	}

	lg.Debugf("After ParseTreeVisitor:\n%s", ToTreeString(v.ast))

	insp := NewInspector(v.ast)
	ds, err := insp.findDatasource()
	if err != nil {
		return nil, err
	}
	v.ast.Datasource = ds

	err = NewWalker(v.ast).AddVisitor(TypeSelector, narrowTblSel).Walk()
	if err != nil {
		return nil, err
	}
	lg.Debugf("After narrowTblSel:\n%s", ToTreeString(v.ast))

	err = NewWalker(v.ast).AddVisitor(TypeSelector, narrowColSel).Walk()
	if err != nil {
		return nil, err
	}
	lg.Debugf("After narrowColSel:\n%s", ToTreeString(v.ast))

	err = NewWalker(v.ast).AddVisitor(TypeFnJoin, determineJoinTables).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(v.ast).AddVisitor(TypeRowRange, visitCheckRowRange).Walk()
	if err != nil {
		return nil, err
	}

	lg.Debugf("After AST visitors:\n%s", ToTreeString(v.ast))

	return v.ast, nil

}

// Model builds a SelectStmt from the IR.
func BuildModel(ast *AST) (*SelectStmt, error) {

	lg.Debugf("starting to build model")
	m := &modeler{ast: ast}

	stmt, err := m.build2()
	if err != nil {
		return nil, err
	}

	lg.Debugf("got stmt from build2: %v", stmt)

	//stmt2, err := m.build()
	//if err != nil {
	//	return nil, nil, err
	//}

	return stmt, nil
	//return stmt2, stmt, nil
}

type modeler struct {
	ast *AST
	//stmt *translator.SelectStmt
}

func (m *modeler) build2() (*SelectStmt, error) {

	lg.Debugf("starting build2()")
	ast := m.ast
	if len(ast.Segments()) == 0 {
		return nil, errorf("parse error: the query does not have enough segments")
	}

	stmt := &SelectStmt{AST: ast}

	insp := NewInspector(ast)
	selectableSeg, err := insp.findFinalSelectableSegment()
	if err != nil {
		return nil, err
	}

	if len(selectableSeg.Children()) != 1 {
		return nil, errorf("the final selectable segment must have exactly one selectable element, but found %d elements", len(selectableSeg.Children()))
	}

	selectable, ok := selectableSeg.Children()[0].(Selectable)
	if !ok {
		return nil, errorf("the final selectable segment must have exactly one selectable element, but found element %T(%q)", selectableSeg.Children()[0], selectableSeg.Children()[0].Text())
	}

	stmt.Selectable = selectable
	lg.Debugf("found selectable segment: %q", selectableSeg.Text())

	// Look for range
	for seg := selectableSeg.Next(); seg != nil; seg = seg.Next() {

		childType, err := seg.ChildType()
		if err != nil {
			return nil, err
		}

		if childType == TypeRowRange {
			if len(seg.Children()) != 1 {
				return nil, errorf("segment [%d] with row range must have exactly one element, but found %d: %q", seg.SegIndex(), len(seg.Children()), seg.Text())
			}

			rr, ok := seg.Children()[0].(*RowRange)
			if !ok {
				return nil, errorf("expected row range, but got %T(%q)", seg.Children()[0], seg.Children()[0].Text())
			}

			if stmt.Range != nil {
				return nil, errorf("only one row range permitted, but found %q and %q", stmt.Range.Text(), rr.Text())
			}

			lg.Debugf("found row range: %q", rr.Text())
			stmt.Range = rr
		}
	}

	seg, err := insp.findColExprSegment()
	if err != nil {
		return nil, err
	}

	if seg == nil {
		lg.Debugf("did not find a col expr segment")
	} else {
		lg.Debugf("found col expr segment: %s", seg.Text())
		elems := seg.Children()
		colExprs := make([]ColExpr, len(elems))
		for i, elem := range elems {
			colExpr, ok := elem.(ColExpr)
			if !ok {
				return nil, errorf("expected element in segment [%d] to be col expr, but was %T", i, elem)
			}

			colExprs[i] = colExpr
		}

		stmt.Cols = colExprs
	}

	// the column expressions should be the last seg
	//seg := ast.Segments()[len(ast.Segments())-1]
	//
	//childTyp, err := seg.ChildType()
	//if err != nil {
	//	return nil, err
	//}

	return stmt, nil
}

//
//func (m *modeler) build() (*translator.SelectStmt, error) {
//	ast := m.ast
//	//stmt := &translator.SelectStmt{}
//	stmt := m.stmt
//
//	if len(ast.Segments()) == 0 {
//		return nil, errorf("no query segments found")
//	}
//
//	//for _, seg := range ir.Segments() {
//	//
//	//	typ, err := seg.ChildType()
//	//	if err != nil {
//	//		return err
//	//	}
//	//
//	//	switch typ {
//	//
//	//	case Type
//	//
//	//
//	//	}
//	//
//	//}
//
//	if len(ast.Segments()) < 2 {
//		return nil, errorf("parse error: the query does not have enough segments")
//	}
//
//	// skip over the first segment, it just contains the datasource, which we already have
//	seg := ast.Segments()[1]
//
//	if len(seg.Children()) == 0 {
//		return nil, errorf("segment[%d] must include at least 1 table selector", seg.SegIndex())
//	}
//
//	//sel, ok := firstSeg.Children()[0].(*Selector)
//	//if !ok {
//	//	return nil, newIRError("expected %s but got %T", TypeSelector, firstSeg.Children()[0])
//	//}
//
//	tblSels := []string{}
//
//	for i, child := range seg.Children() {
//
//		sel, ok := child.(*TblSelector)
//		if !ok {
//			return nil,
//				errorf("segment[%d]: element[%d]: expected %s but got %T(%s)", // fixme, should use text location
//					0, i,
//					TypeTableSelector, seg.Children()[i], seg.Children()[i].Text())
//		}
//
//		tblSels = append(tblSels, sel.SelValue())
//
//	}
//
//	stmt.Tables = tblSels
//	seg.consumed = true
//
//	for _, seg := range ast.Segments()[2:] {
//
//		typ, err := seg.ChildType()
//		if err != nil {
//			return nil, err
//		}
//
//		switch typ {
//
//		case TypeFnJoin:
//			err := m.handleJoin(seg.Children()[0].(*FnJoin))
//			if err != nil {
//				return nil, err
//			}
//		case TypeSelector:
//			err := m.handleColSelSegment(seg)
//			if err != nil {
//				return nil, err
//			}
//		case TypeRowRange:
//			err := m.handleRowRange(seg.Children()[0].(*RowRange))
//			if err != nil {
//				return nil, err
//			}
//
//		}
//
//	}
//
//	seg = seg.Next()
//
//	if len(stmt.Cols) == 0 {
//		stmt.Cols = []string{"*"}
//	}
//
//	return stmt, nil
//}
//
//func (m *modeler) handleRowRange(rr *RowRange) error {
//
//	lg.Debugf("starting...")
//	m.stmt.Offset = rr.offset
//	m.stmt.Limit = rr.limit
//
//	return nil
//}
//
//func (m *modeler) handleColSelSegment(seg *Segment) error {
//	lg.Debugf("starting...")
//	// otherwise we must have a sel
//	colExprs := []string{}
//	for _, child := range seg.Children() {
//
//		sel, ok := child.(*Selector)
//		if !ok {
//			return errorf("expected *Selector but got %T", child)
//		}
//
//		colExprs = append(colExprs, sel.Text()[1:])
//	}
//
//	m.stmt.Cols = colExprs
//	return nil
//}
//
//func (m *modeler) handleJoin(joinNode *FnJoin) error {
//	lg.Debugf("starting...")
//	if len(joinNode.Children()) == 0 {
//		return errorf("JOIN requires arguments, none provided")
//	}
//
//	joinExpr, ok := joinNode.Children()[0].(*FnJoinExpr)
//	if !ok {
//		return errorf("expected *FnJoinExpr but got %T", joinNode.Children()[0])
//	}
//
//	// we've got a join
//	// get the table we're joining to
//	joinTblName := m.stmt.Tables[1]
//	leftOperand := ""
//	operator := ""
//	rightOperand := ""
//
//	if len(joinExpr.Children()) == 1 {
//		// It's a single col selector
//		colSel, ok := joinExpr.Children()[0].(*ColSelector)
//		if !ok {
//			return errorf("expected *Selector but got %T", joinExpr.Children()[0])
//		}
//
//		leftOperand = fmt.Sprintf("`%s`.`%s`", m.stmt.Tables[0], colSel.SelValue())
//		operator = "=="
//		rightOperand = fmt.Sprintf("`%s`.`%s`", m.stmt.Tables[1], colSel.SelValue())
//	} else {
//		leftOperand = joinExpr.Children()[0].Text()[1:]
//		operator = joinExpr.Children()[1].Text()
//		rightOperand = joinExpr.Children()[2].Text()[1:]
//	}
//
//	if operator == "==" {
//		operator = "="
//	}
//
//	joinText := fmt.Sprintf("JOIN `%s` ON %s %s %s", joinTblName, leftOperand, operator, rightOperand)
//	lg.Debugf("%q", joinText)
//	m.stmt.Joins = append(m.stmt.Joins, joinText)
//	return nil
//}

type SelectStmt struct {
	AST        *AST
	Selectable Selectable
	//Cols       []*ColSelector
	Cols  []ColExpr
	Range *RowRange
}

func (s *SelectStmt) String() string {
	return fmt.Sprintf("%v | %v  |  %v", s.Selectable, s.Cols, s.Range)
}

//seg = seg.Next()
//if seg == nil {
//// no more segments, this is basically a "SELECT * FROM tbl"
//stmt.Cols = []string{"*"}
//return stmt, nil
//}
//
//// we have a segment, check if it's a join
//child := seg.Children()[0]
//joinNode, ok := child.(*FnJoin)
//if ok {
//
//if len(joinNode.Children()) == 0 {
//return nil, newIRError("JOIN requires arguments, none provided")
//}
//
//joinExpr, ok := joinNode.Children()[0].(*FnJoinExpr)
//if !ok {
//return nil, newIRError("expected *FnJoinExpr but got %T", joinNode.Children()[0])
//}
//
//// we've got a join
//// get the table we're joining to
//joinTblName := stmt.Tables[1]
//leftOperand := ""
//operator := ""
//rightOperand := ""
//
//if len(joinExpr.Children()) == 1 {
//// It's a single col selector
//colSel, ok := joinExpr.Children()[0].(*Selector)
//if !ok {
//return nil, newIRError("expected *Selector but got %T", joinExpr.Children()[0])
//}
//
//leftOperand = fmt.Sprintf("`%s`.`%s`", stmt.Tables[0], colSel.SelValue())
//operator = "=="
//rightOperand = fmt.Sprintf("`%s`.`%s`", stmt.Tables[1], colSel.SelValue())
//} else {
//leftOperand = joinExpr.Children()[0].Text()[1:]
//operator = joinExpr.Children()[1].Text()
//rightOperand = joinExpr.Children()[2].Text()[1:]
//}
//
//if operator == "==" {
//operator = "="
//}
//
//joinText := fmt.Sprintf("JOIN `%s` ON %s %s %s", joinTblName, leftOperand, operator, rightOperand)
//lg.Debugf(joinText)
//stmt.Joins = append(stmt.Joins, joinText)
//seg.consumed = true
//seg = seg.Next()
//}
//
//if seg == nil {
//// no more segments, this is basically a "SELECT * FROM tbl"
//stmt.Cols = []string{"*"}
//return stmt, nil
//}
//
//// otherwise we must have a sel
//colExprs := []string{}
//for _, child := range seg.Children() {
//
//sel, ok := child.(*Selector)
//if !ok {
//return nil, newIRError("expected *Selector but got %T", child)
//}
//
//colExprs = append(colExprs, sel.Text()[1:])
//}
//
//stmt.Cols = colExprs
//
//return stmt, nil
