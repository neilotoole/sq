package ql

import (
	"strconv"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/ql/parser"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

type ParseTreeVisitor struct {
	Err       error
	listenFns []func(antlr.ParseTree) (bool, error)
	cur       Node
	ast       *AST
}

func (v *ParseTreeVisitor) notify(ctx antlr.ParseTree) bool {

	for i := 0; i < len(v.listenFns); i++ {

		listenFn := v.listenFns[i]
		ok, err := listenFn(ctx)
		if err != nil {
			v.Err = err
		}
		if !ok {
			return false
		}
	}

	return true
}

func (v *ParseTreeVisitor) AddListener(fn func(antlr.ParseTree) (bool, error)) {
	v.listenFns = append(v.listenFns, fn)
}

func (v *ParseTreeVisitor) Visit(ctx antlr.ParseTree) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	switch ctx := ctx.(type) {

	case *parser.SegmentContext:
		v.VisitSegment(ctx)
	case *parser.ElementContext:
		v.VisitElement(ctx)
	case *parser.DsElementContext:
		v.VisitDsElement(ctx)
	case *parser.DsTblElementContext:
		v.VisitDsTblElement(ctx)
	case *parser.SelElementContext:
		v.VisitSelElement(ctx)
	case *parser.FnContext:
		v.VisitFn(ctx)
	case *parser.FnJoinContext:
		v.VisitFnJoin(ctx)
	case *parser.FnJoinExprContext:
		v.VisitFnJoinExpr(ctx)
	case *parser.FnJoinCondContext:
		v.VisitFnJoinCond(ctx)
	case *parser.CmprContext:
		v.VisitCmpr(ctx)
	case *parser.RowRangeContext:
		v.VisitRowRange(ctx)
	case *antlr.TerminalNodeImpl:
		v.VisitTerminal(ctx)

	default:
		err := errorf("unknown node type: %T", ctx)
		v.Err = err
	}

	return nil
	//result := visit(v, ctx)
	//if result != nil {
	//	if err, ok := result.(error); ok {
	//		v.Err = err
	//	}
	//}
	//return result
}

func (v *ParseTreeVisitor) VisitChildren(ctx antlr.RuleNode) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	for _, child := range ctx.GetChildren() {
		if v.Err != nil {
			return nil
		}

		tree, ok := child.(antlr.ParseTree)
		if !ok {
			v.Err = errorf("unknown child node type: %T %q", child, child.GetPayload())
		}

		v.Visit(tree)

	}
	return nil
}

func (v *ParseTreeVisitor) VisitQuery(ctx *parser.QueryContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	//allSegs := ctx.AllSegment()

	// datasource segment, i.e. the first segment
	//dsSeg := ctx.AllSegment()[0].(*parser.SegmentContext)
	//
	//if len(dsSeg.AllElement()) != 1 {
	//	v.Err = newIRError("first segment must contain exactly one datasource")
	//	return nil
	//}
	//
	//dsElem := dsSeg.AllElement()[0].(*parser.ElementContext).DatasourceElement()
	//if dsElem == nil {
	//	v.Err = newIRError("query datasource not found in first segment")
	//	return nil
	//}
	//
	//ds := dsElem.GetText()
	////
	////ds := ctx.DS().GetText()
	////if ds == "" {
	////	v.Err = newIRError("query datasource not found")
	////	return nil
	////}
	//
	//lg.Debugf("found datasource: %q", ds)
	//v.ir = &IR{Datasource: ds}
	v.ast = &AST{}
	v.ast.ctx = ctx
	//v.ir.SetContext(ctx)
	v.cur = v.ast

	for _, seg := range ctx.AllSegment() {
		if v.Err != nil {
			return nil
		}
		v.VisitSegment(seg.(*parser.SegmentContext))
	}

	return nil
}

func (v *ParseTreeVisitor) VisitDsElement(ctx *parser.DsElementContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	ds := &Datasource{}
	ds.parent = v.cur
	ds.ctx = ctx.DATASOURCE()
	v.cur.AddChild(ds)
	return nil
}

func (v *ParseTreeVisitor) VisitDsTblElement(ctx *parser.DsTblElementContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	tblSel := &TblSelector{}
	tblSel.parent = v.cur
	tblSel.ctx = ctx

	tblSel.dsName = ctx.DATASOURCE().GetText()
	tblSel.tblName = ctx.SEL().GetText()[1:]

	v.cur.AddChild(tblSel)
	//ds := &Datasource{}
	//ds.parent = v.cur
	//ds.ctx = ctx.DATASOURCE()
	//v.cur.AddChild(ds)
	return nil
}

func (v *ParseTreeVisitor) VisitSegment(ctx *parser.SegmentContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	seg := &Segment{}
	seg.bn.ctx = ctx
	seg.bn.parent = v.ast
	//seg.ir = v.ir
	v.ast.AddSegment(seg)

	v.cur = seg

	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitSelElement(ctx *parser.SelElementContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	//selNode := ctx.SEL()
	//selector := &Selector{parent: v.cur, ctx: selNode}
	selector := &Selector{}
	selector.parent = v.cur
	selector.ctx = ctx.SEL()
	//selector := &Selector{parent: v.cur, ctx: selNode}
	v.cur.AddChild(selector)

	//return v.VisitChildren(ctx)
	return nil
}

func (v *ParseTreeVisitor) VisitElement(ctx *parser.ElementContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}
	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitCmpr(ctx *parser.CmprContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}
	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitFn(ctx *parser.FnContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitArgs(ctx *parser.ArgsContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}
	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitArg(ctx *parser.ArgContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}
	return v.VisitChildren(ctx)
}

func (v *ParseTreeVisitor) VisitFnJoin(ctx *parser.FnJoinContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	// parent node must be a segment
	seg, ok := v.cur.(*Segment)
	if !ok {
		v.Err = errorf("parent of JOIN() must be Segment, but got: %T", v.cur)
		return nil
	}

	join := &FnJoin{seg: seg, ctx: ctx}
	seg.AddChild(join)

	expr := ctx.FnJoinExpr()
	if expr == nil {
		return nil
	}

	// the join contains an expr, let's hit it
	v.cur = join
	v.VisitFnJoinExpr(expr.(*parser.FnJoinExprContext))

	// set cur back to previous
	v.cur = seg

	return nil
}

func (v *ParseTreeVisitor) VisitFnJoinExpr(ctx *parser.FnJoinExprContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	// the expression could be empty
	children := ctx.GetChildren()
	if len(children) == 0 {
		return nil
	}

	// it could be either a join condition, or just a SEL
	cond := ctx.FnJoinCond()
	if cond != nil {
		return v.VisitFnJoinCond(cond.(*parser.FnJoinCondContext))
	}

	joinNode, ok := v.cur.(*FnJoin)
	if !ok {
		v.Err = errorf("JOIN condition must have JOIN parent, but got %T", v.cur)
	}

	sel := ctx.SEL()
	if sel == nil {
		// shouldn't happen, parser should have caught this beforehand
		v.Err = errorf("invalid JOIN expression: %q", ctx.GetText())
		return nil
	}

	joinExprNode := &FnJoinExpr{join: joinNode, ctx: ctx}

	colSelNode := &Selector{}
	colSelNode.ctx = sel
	colSelNode.parent = joinExprNode

	joinExprNode.AddChild(colSelNode)
	joinNode.AddChild(joinExprNode)

	return nil
}

func (v *ParseTreeVisitor) VisitFnJoinCond(ctx *parser.FnJoinCondContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	// the expression could be empty
	children := ctx.GetChildren()
	if len(children) == 0 {
		return nil
	}

	// or else it's a LEFT CMPR RIGHT, e.g. .user.uid == .address.uid

	sels := ctx.AllSEL()
	if len(sels) != 2 {
		// REVISIT: probably unnecessary, should be caught by the parser
		v.Err = errorf("JOIN condition must have 2 operands (left & right), but got %d", len(sels))
		return nil
	}

	join, ok := v.cur.(*FnJoin)
	if !ok {
		v.Err = errorf("JOIN condition must have JOIN parent, but got %T", v.cur)
	}
	joinCondition := &FnJoinExpr{join: join, ctx: ctx}

	leftSel := &Selector{}
	leftSel.ctx = sels[0]
	leftSel.parent = joinCondition

	rightSel := &Selector{}
	rightSel.ctx = sels[1]
	rightSel.parent = joinCondition

	cmpr := NewCmpr(joinCondition, ctx.Cmpr())

	v.setIfErr(joinCondition.AddChild(leftSel))
	v.setIfErr(joinCondition.AddChild(cmpr))
	v.setIfErr(joinCondition.AddChild(rightSel))
	v.setIfErr(join.AddChild(joinCondition))

	return nil
}

func (v *ParseTreeVisitor) VisitTerminal(ctx antlr.TerminalNode) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	//tk := ctx.GetSymbol()
	//lg.Debugf(fmt.Sprintf("[%d:%d:%d] %q", tk.GetLine(), tk.GetStart(), tk.GetStop(), tk.GetText()))

	return nil
}

func (v *ParseTreeVisitor) VisitRowRange(ctx *parser.RowRangeContext) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}

	// []      select all rows (no range)
	// [1]     select row[1]
	// [10:15] select rows 10 thru 15
	// [0:15]  select rows 0 thru 15
	// [:15]   same as above (0 thru 15)
	// [10:]   select all rows from 10 onwards

	//lg.Debugf("ctx text[children: %d]: %q\n", len(ctx.GetChildren()), ctx.GetText())

	if ctx.COLON() == nil && len(ctx.AllINT()) == 0 {
		// [] select all rows, aka no range
		return nil
	}

	if ctx.COLON() == nil {
		// [1] -- select row[1]
		if len(ctx.AllINT()) != 1 {
			return errorf("row range: expected one integer but got %d", len(ctx.AllINT()))
		}

		i, _ := strconv.Atoi(ctx.AllINT()[0].GetText())
		rr := NewRowRange(ctx, i, 1)
		return v.cur.AddChild(rr)
	}

	// there's a colon... can only be one or two ints
	if len(ctx.AllINT()) > 2 {
		return errorf("row range: expected one or two integers but got %d", len(ctx.AllINT()))
	}

	if len(ctx.AllINT()) == 2 {
		// [10:15] -- select rows 10 thru 15
		offset, _ := strconv.Atoi(ctx.AllINT()[0].GetText())
		finish, _ := strconv.Atoi(ctx.AllINT()[1].GetText())
		limit := finish - offset
		rr := NewRowRange(ctx, offset, limit)
		return v.cur.AddChild(rr)

	}

	// it's one of these two cases:
	//   [:15]   (0 thru 15)
	//   [10:]   select all rows from 10 onwards
	// so we need to determine if the INT is before or after the colon
	offset := -1
	limit := -1

	if ctx.COLON().GetSymbol().GetTokenIndex() < ctx.AllINT()[0].GetSymbol().GetTokenIndex() {
		// [:15]   (0 thru 15)
		offset = 0
		limit, _ = strconv.Atoi(ctx.AllINT()[0].GetText())
	} else {
		// [10:]   select all rows from 10 onwards
		offset, _ = strconv.Atoi(ctx.AllINT()[0].GetText())
	}

	rr := NewRowRange(ctx, offset, limit)
	return v.cur.AddChild(rr)
	//for _, child := range ctx.GetChildren() {
	//
	//	node := child.(*antlr.TerminalNodeImpl)
	//	node.GetSymbol().
	//
	//}

	//
	//	offset := -1
	//limit := -1
	//
	//for _, item := range ctx.GetChildren() {
	//
	//}

	//return nil
}

func (v *ParseTreeVisitor) VisitErrorNode(ctx antlr.ErrorNode) interface{} {
	ok := v.notify(ctx)
	if !ok {
		return nil
	}
	return nil
}

func LogNodeVisit(ctx antlr.ParseTree) (bool, error) {

	txt := ctx.GetText()
	if len(txt) > 256 {
		txt = txt[0:253] + "..."
	}

	//lg.DebugN(2, "%T %q\n", ctx, txt)
	lg.Depth(2).Debugf("%T %q\n", ctx, txt)
	return true, nil
}

func (v *ParseTreeVisitor) setIfErr(err error) error {
	if err != nil {
		v.Err = err
	}
	return err
}
