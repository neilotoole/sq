package ql

import (
	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/ql/parser"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// AST is sq's Abstract Syntax Tree.
type AST struct {
	//bn BaseNode
	ctx        *parser.QueryContext
	Datasource string
	segs       []*Segment
}

func (a *AST) Parent() Node {
	return nil
}

func (a *AST) SetParent(parent Node) error {
	return errorf("root node (%T) cannot have parent: tried to add parent %T", a, parent)
}

func (a *AST) Children() []Node {
	nodes := make([]Node, len(a.segs))

	for i, seg := range a.segs {
		nodes[i] = seg
	}

	return nodes
	//return []Node(a.Segments)
	//return a.bn.Children()
}

func (a *AST) Segments() []*Segment {
	//segs := make([]*Segment, len(a.bn.Children()))
	//for i, seg := range a.bn.Children() {
	//	segs[i] = seg.(*Segment)
	//}

	return a.segs
	//
	//return nodes
	//return []Node(a.Segments)
	//return a.bn.Children()
}

func (a *AST) AddChild(node Node) error {

	seg, ok := node.(*Segment)
	if !ok {
		return errorf("expected *Segment but got: %T", node)
	}

	a.AddSegment(seg)
	//return a.bn.AddChild(seg)
	return nil
}

func (a *AST) SetChildren(children []Node) error {

	segs := make([]*Segment, len(children))

	for i, child := range children {
		seg, ok := child.(*Segment)
		if !ok {
			return errorf("expected child of type %s, but got: %T", TypeSegment, child)
		}

		segs[i] = seg
	}

	a.segs = segs
	return nil
}

func (a *AST) Context() antlr.ParseTree {
	return a.ctx
}

func (a *AST) SetContext(ctx antlr.ParseTree) error {

	qCtx, ok := ctx.(*parser.QueryContext)
	if !ok {
		return errorf("expected *parser.QueryContext, but got %T", ctx)
	}

	a.ctx = qCtx
	return nil
}

//func (ir *IR) Replace(swap Node) error {
//	return newIRError("%T can't be replaced, especially not by %T", ir, swap)
//}

func (a *AST) String() string {

	//return fmt.Sprint("%T: %s", ir, )
	//return fmt.Sprintf("%T: [%d segments]", ir, len(a.segs))

	return nodeString(a)
	//str := make([]string, len(a.Segments)+1)
	//str[0] = a.Datasource
	//
	//for i, seg := range a.Segments {
	//	str[i+1] = seg.String()
	//}
	//
	//return strings.Join(str, " | ")
}

func (a *AST) Text() string {
	return a.ctx.GetText()
}

// AddSegment appends seg to the IR.
func (a *AST) AddSegment(seg *Segment) {
	seg.SetParent(a)

	//a.bn.AddChild(seg)

	a.segs = append(a.segs, seg)
}

// Build creates an IR from the query.
//func Build(q *parser.QueryContext) (*IR, error) {
//func Build(query parser.IQueryContext) (*IR, error) {
//
//	q, ok := query.(*parser.QueryContext)
//	if !ok {
//		return nil, newIRError("expected *parser.QueryContext but got %T", query)
//	}
//	// Example:
//	//  `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
//
//	ir := &IR{Datasource: q.DS().GetText(), ctx: q}
//
//	for _, seg := range q.AllSegment() {
//		a.AddSegment(&Segment{ctx: seg.(*parser.SegmentContext)})
//	}
//
//	for _, seg := range a.Segments {
//		err := a.processSegment(seg)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	return ir, nil
//}

//func doSelector
//
//func (ir *IR) processSegment(seg *Segment) error {
//
//	//_, err := seg.HasCompatibleNodes()
//	//if err != nil {
//	//	return err
//	//}
//
//	typ, err := seg.getNodesElementType()
//	if err != nil {
//		return err
//	}
//
//	lg.Debugf("seg[%d] has node element type: %q\n", seg.Index(), typ)
//
//	_ = seg.Prev()
//
//	// complete and utter hack
//	//if typ.String() == "a.FnJoin" {
//	//
//	//}
//	//
//	//if typ == TypeFnJoin {
//	//	fmt.Printf("found FnJoin\n")
//	//}
//	//
//	//if parent == nil {
//	//	//  `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
//	//	// This is the first segment (.user, .address), therefore the
//	//	// elements should be table selectors.
//	//	for _, item := range seg.ctx.AllElement() {
//	//
//	//		e := item.(*parser.ElementContext)
//	//
//	//		sel := e.SEL()
//	//		if sel == nil {
//	//			return fmt.Errorf("expected table selector, but got %q", e.GetText())
//	//		}
//	//
//	//		//fmt.Printf("SEL: %T(%s)\n", sel, sel.GetText())
//	//
//	//		seg.AddElement(NewTableSelector(sel.GetText(), seg, sel))
//	//
//	//		//child := e.GetChild(0)
//	//		//fmt.Printf("child: %T(%s)\n", child, child)
//	//		//
//	//		//leafNode := child.(*antlr.TerminalNodeImpl)
//	//		//fmt.Printf("node: %T(%s)\n", leafNode, leafNode.GetText())
//	//		//symbol := leafNode.GetSymbol()
//	//		//fmt.Printf("symbol: %T(%s)\n", symbol, symbol)
//	//
//	//	}
//	//
//	//	return nil
//	//}
//
//	// it's not the root, treat selectors as column selectors
//	for _, item := range seg.ctx.AllElement() {
//
//		e := item.(*parser.ElementContext).GetChild(0)
//
//		if leaf, ok := e.(*antlr.TerminalNodeImpl); ok {
//
//			tok := leaf.GetSymbol()
//			switch tok.GetTokenType() {
//			case parser.SQLexerSEL:
//				err := seg.AddChild(NewColSelector(seg, leaf))
//				if err != nil {
//					return err
//				}
//				//fmt.Printf("it's a SEL\n")
//			default:
//				return newIRError("unexpected token: %q", tok)
//			}
//		}
//
//		//sel := e.SEL()
//		//if sel == nil {
//		//	return fmt.Errorf("expected table selector, but got %q", e.GetText())
//		//}
//		//
//		////fmt.Printf("SEL: %T(%s)\n", sel, sel.GetText())
//		//
//		//seg.AddElement(NewTableSelector(sel.GetText(), seg, sel))
//
//		//child := e.GetChild(0)
//		//fmt.Printf("child: %T(%s)\n", child, child)
//		//
//		//leafNode := child.(*antlr.TerminalNodeImpl)
//		//fmt.Printf("node: %T(%s)\n", leafNode, leafNode.GetText())
//		//symbol := leafNode.GetSymbol()
//		//fmt.Printf("symbol: %T(%s)\n", symbol, symbol)
//
//	}
//
//	//rootSeg := segs[0].(*parser.SegmentContext)
//
//	return nil
//}

//
//// Model builds a SelectStmt from the IR.
//func (ir *IR) Model() (*translator.SelectStmt, error) {
//
//	stmt := &translator.SelectStmt{}
//
//	finalSeg := a.Segments[len(a.Segments)-1]
//	finalFoundColExpr := false
//	finalFoundOther := false
//	// check that the segment is all ColExpr
//	for _, elem := range finalSeg.Elements {
//		if _, ok := elem.(ColExprer); ok {
//			finalFoundColExpr = true
//			continue
//		}
//		finalFoundOther = true
//	}
//
//	if finalFoundOther == true && finalFoundColExpr == true {
//		return nil, newIRError("can't mix column selectors and other operations")
//	}
//
//	if finalFoundOther == false {
//		// this means the elements are all column selectors, which is normal
//		for _, elem := range finalSeg.Elements {
//			colExpr := elem.(ColExprer)
//			ce, err := colExpr.ColExpr()
//			if err != nil {
//				return nil, newIRError("column select error: %s", err)
//			}
//			stmt.Cols = append(stmt.Cols, ce)
//			finalSeg.consumed = true
//		}
//	} else {
//		// this means that the last segment doesn't include any ColExpr,
//		// therefore we're likely selecting all elements "*"
//		stmt.Cols = append(stmt.Cols, "*")
//		// little hack to indicate that we're not done with this row
//		finalSeg.consumed = false
//
//	}
//
//	// start from the bottom, and work our way up the segments.
//	// We've already taken the column selectors, so we're looking for a segment
//	// that has (as of current impl) exactly one Fromer element
//
//	for i := len(a.Segments) - 1; i >= 0; i-- {
//		seg := a.Segments[i]
//		if seg.consumed {
//			continue
//		}
//
//		if len(seg.Elements) != 1 {
//			continue
//		}
//
//		// so, there's only one element in this segment, check if it
//		// implements Fromer
//		fromer, ok := seg.Elements[0].(Fromer)
//		if !ok {
//			continue
//		}
//
//		// we've found a single element, and it implements Fromer, so that's
//		// our From statement.
//		from, err := fromer.From()
//		if err != nil {
//			return nil, err
//		}
//
//		stmt.Table = from
//	}
//
//	if stmt.Table == "" {
//		return nil, newIRError("unable to determine FROM clause")
//	}
//
//	//cols := []string{}
//
//	return stmt, nil
//}

type Error struct {
	msg string
}

func (e *Error) Error() string {
	return e.msg
}

func errorf(format string, v ...interface{}) *Error {
	err := &Error{msg: fmt.Sprintf(format, v...)}
	lg.Depth(1).Warnf("error created: %s", err.msg)
	return err
}
