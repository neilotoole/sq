package ast

import (
	"fmt"

	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

type Fn interface {
	FnName() string
}

type Join struct {
	seg        *Segment
	ctx        antlr.ParseTree
	constraint *JoinConstraint
	leftTbl    *TblSelector
	rightTbl   *TblSelector
}

func (jn *Join) LeftTbl() *TblSelector {
	return jn.leftTbl
}
func (jn *Join) RightTbl() *TblSelector {
	return jn.rightTbl
}
func (jn *Join) Selectable() {
	// no-op
}

func (jn *Join) Parent() Node {
	return jn.seg
}

func (jn *Join) SetParent(parent Node) error {

	seg, ok := parent.(*Segment)
	if !ok {
		return errorf("%T requires parent of type %s", jn, TypeSegment)
	}
	jn.seg = seg
	return nil
}

func (jn *Join) Children() []Node {

	if jn.constraint == nil {
		return []Node{}
	}

	return []Node{jn.constraint}
}

func (jn *Join) AddChild(node Node) error {

	expr, ok := node.(*JoinConstraint)
	if !ok {
		return errorf("JOIN() child must be *FnJoinExpr, but got: %T", node)
	}

	if jn.constraint != nil {
		return errorf("JOIN() has max 1 child: failed to add: %T", node)
	}

	jn.constraint = expr
	return nil
}

func (jn *Join) SetChildren(children []Node) error {

	if len(children) == 0 {
		jn.constraint = nil
		return nil
	}

	if len(children) > 1 {
		return errorf("JOIN() can have only one child: failed to add %d children", len(children))
	}

	expr, ok := children[0].(*JoinConstraint)
	if !ok {
		return errorf("JOIN() child must be *FnJoinExpr, but got: %T", children[0])
	}

	jn.constraint = expr
	return nil
}

func (jn *Join) Context() antlr.ParseTree {
	return jn.ctx
}

func (jn *Join) SetContext(ctx antlr.ParseTree) error {
	jn.ctx = ctx
	return nil
}

func (jn *Join) Text() string {
	return jn.ctx.GetText()
}

func (jn *Join) Segment() *Segment {
	return jn.seg
}

//func (jn *Join) FnName() string {
//	return "JOIN"
//}

func (jn *Join) String() string {
	text := nodeString(jn)

	leftTblName := ""
	rightTblName := ""

	if jn.leftTbl != nil {
		leftTblName = jn.leftTbl.SelValue()
	}
	if jn.rightTbl != nil {
		rightTblName = jn.rightTbl.SelValue()
	}

	text = text + fmt.Sprintf(" |  left_table: %q  |  right_table: %q", leftTblName, rightTblName)
	return text
}

//func NewJoinFn(seg *Segment, ctx antlr.ParseTree) *Join {
//	j := &Join{seg: seg, ctx: ctx}
//	return j
//}

type JoinConstraint struct {
	join     *Join
	ctx      antlr.ParseTree
	children []Node
}

func (jc *JoinConstraint) Parent() Node {
	return jc.join
}

func (jc *JoinConstraint) SetParent(parent Node) error {

	join, ok := parent.(*Join)
	if !ok {
		return errorf("%T requires parent of type %s", jc, TypeFnJoin)
	}
	jc.join = join
	return nil
}

func (jc *JoinConstraint) Children() []Node {
	return jc.children
}

func (jc *JoinConstraint) AddChild(child Node) error {

	nodeCtx := child.Context()
	_, ok := nodeCtx.(*antlr.TerminalNodeImpl)
	if !ok {
		return errorf("expected leaf node, but got: %T", nodeCtx)
	}

	jc.children = append(jc.children, child)
	return nil
}

func (jc *JoinConstraint) SetChildren(children []Node) error {

	if len(children) == 0 {
		jc.children = children
		return nil
	}

	for _, child := range children {

		nodeCtx := child.Context()
		_, ok := nodeCtx.(*antlr.TerminalNodeImpl)
		if !ok {
			return errorf("expected leaf node, but got: %T", nodeCtx)
		}
	}

	jc.children = children
	return nil
}

func (jc *JoinConstraint) Context() antlr.ParseTree {
	return jc.ctx
}

func (jc *JoinConstraint) SetContext(ctx antlr.ParseTree) error {

	jc.ctx = ctx // TODO: check for correct type
	return nil
}

func (jc *JoinConstraint) Text() string {
	return jc.ctx.GetText()
}

func (jc *JoinConstraint) String() string {
	return nodeString(jc)
}
