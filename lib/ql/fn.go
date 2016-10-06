package ql

import (
	"fmt"

	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

type Fn interface {
	FnName() string
}

type FnJoin struct {
	seg      *Segment
	ctx      antlr.ParseTree
	expr     *FnJoinExpr
	leftTbl  *TblSelector
	rightTbl *TblSelector
}

func (fn *FnJoin) Selectable() {
	// no-op
}

func (fn *FnJoin) Parent() Node {
	return fn.seg
}

func (fn *FnJoin) SetParent(parent Node) error {

	seg, ok := parent.(*Segment)
	if !ok {
		return errorf("%T requires parent of type %s", fn, TypeSegment)
	}
	fn.seg = seg
	return nil
}

//func (fn *FnJoin) Replace(swap Node) error {
//	return newIRError("does not implement")
//}

func (fn *FnJoin) Children() []Node {

	if fn.expr == nil {
		return []Node{}
	}

	return []Node{fn.expr}
}

func (fn *FnJoin) AddChild(node Node) error {

	expr, ok := node.(*FnJoinExpr)
	if !ok {
		return errorf("JOIN() child must be *FnJoinExpr, but got: %T", node)
	}

	if fn.expr != nil {
		return errorf("JOIN() has max 1 child: failed to add: %T", node)
	}

	fn.expr = expr
	return nil
}

func (fn *FnJoin) SetChildren(children []Node) error {

	if len(children) == 0 {
		fn.expr = nil
		return nil
	}

	if len(children) > 1 {
		return errorf("JOIN() can have only one child: failed to add %d children", len(children))
	}

	expr, ok := children[0].(*FnJoinExpr)
	if !ok {
		return errorf("JOIN() child must be *FnJoinExpr, but got: %T", children[0])
	}

	fn.expr = expr
	return nil
}

func (fn *FnJoin) Context() antlr.ParseTree {
	return fn.ctx
}

func (fn *FnJoin) SetContext(ctx antlr.ParseTree) error {
	fn.ctx = ctx
	return nil
}

func (fn *FnJoin) Text() string {
	return fn.ctx.GetText()
}

//func (fn *FnJoin) Value() string {
//	return fn.ctx.GetText()
//}

func (fn *FnJoin) Segment() *Segment {
	return fn.seg
}

func (fn *FnJoin) FnName() string {
	return "JOIN"
}

func (fn *FnJoin) String() string {
	text := nodeString(fn)

	leftTblName := ""
	rightTblName := ""

	if fn.leftTbl != nil {
		leftTblName = fn.leftTbl.SelValue()
	}
	if fn.rightTbl != nil {
		rightTblName = fn.rightTbl.SelValue()
	}

	text = text + fmt.Sprintf(" |  left_table: %q  |  right_table: %q", leftTblName, rightTblName)
	return text
}

//Parent() Node
//Children() []Node
//Context() antlr.ParseTree
//String() string

func NewJoinFn(seg *Segment, ctx antlr.ParseTree) *FnJoin {
	j := &FnJoin{seg: seg, ctx: ctx}
	return j
}

type FnJoinExpr struct {
	join     *FnJoin
	ctx      antlr.ParseTree
	children []Node
}

func (x *FnJoinExpr) Parent() Node {
	return x.join
}

func (x *FnJoinExpr) SetParent(parent Node) error {

	join, ok := parent.(*FnJoin)
	if !ok {
		return errorf("%T requires parent of type %s", x, TypeFnJoin)
	}
	x.join = join
	return nil
}

func (x *FnJoinExpr) Children() []Node {
	return x.children
}

//func (x *FnJoinExpr) Replace(swap Node) error {
//	return newIRError("does not implement")
//}

func (x *FnJoinExpr) AddChild(child Node) error {

	nodeCtx := child.Context()
	_, ok := nodeCtx.(*antlr.TerminalNodeImpl)
	if !ok {
		return errorf("expected leaf node, but got: %T", nodeCtx)
	}

	x.children = append(x.children, child)
	return nil
}

func (x *FnJoinExpr) SetChildren(children []Node) error {

	if len(children) == 0 {
		x.children = children
		return nil
	}

	for _, child := range children {

		nodeCtx := child.Context()
		_, ok := nodeCtx.(*antlr.TerminalNodeImpl)
		if !ok {
			return errorf("expected leaf node, but got: %T", nodeCtx)
		}
	}

	x.children = children
	return nil
}

func (x *FnJoinExpr) Context() antlr.ParseTree {
	return x.ctx
}

func (x *FnJoinExpr) SetContext(ctx antlr.ParseTree) error {

	x.ctx = ctx // TODO: check for correct type
	return nil
}

func (x *FnJoinExpr) Text() string {
	return x.ctx.GetText()
}

//func (fn *FnJoin) Value() string {
//	return fn.ctx.GetText()
//}

//func (x *FnJoinExpr) FnJoin() *FnJoin {
//	return x.join
//}

func (x *FnJoinExpr) String() string {

	return nodeString(x)
	//if len(x.children) == 0 {
	//	return "[empty expr]"
	//}
	//
	//str := make([]string, len(x.children))
	//for i, child := range x.children {
	//	str[i] = child.String()
	//}
	//
	//return strings.Join(str, " ")
}
