package ast

import (
	"fmt"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

var _ Node = (*Join)(nil)

// Join models a SQL JOIN node. It has a child of type JoinConstraint.
type Join struct {
	seg        *Segment
	ctx        antlr.ParseTree
	constraint *JoinConstraint
	leftTbl    *TblSelector
	rightTbl   *TblSelector
}

// LeftTbl is the selector for the left table of the join.
func (jn *Join) LeftTbl() *TblSelector {
	return jn.leftTbl
}

// RightTbl is the selector for the right table of the join.
func (jn *Join) RightTbl() *TblSelector {
	return jn.rightTbl
}

// Selectable implements the Selectable marker interface.
func (jn *Join) Selectable() {
	// no-op
}

func (jn *Join) Parent() Node {
	return jn.seg
}

func (jn *Join) SetParent(parent Node) error {
	seg, ok := parent.(*Segment)
	if !ok {
		return errorf("%T requires parent of type %s", jn, typeSegment)
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
	jc, ok := node.(*JoinConstraint)
	if !ok {
		return errorf("JOIN() child must be *JoinConstraint, but got: %T", node)
	}

	if jn.constraint != nil {
		return errorf("JOIN() has max 1 child: failed to add: %T", node)
	}

	jn.constraint = jc
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

func (jn *Join) String() string {
	text := nodeString(jn)

	leftTblName := ""
	rightTblName := ""

	if jn.leftTbl != nil {
		leftTblName, _ = jn.leftTbl.SelValue()
	}
	if jn.rightTbl != nil {
		rightTblName, _ = jn.rightTbl.SelValue()
	}

	text += fmt.Sprintf(" |  left_table: %q  |  right_table: %q", leftTblName, rightTblName)
	return text
}

var _ Node = (*JoinConstraint)(nil)

// JoinConstraint models a join's constraint.
// For example the elements inside the parentheses
// in "join(.uid == .user_id)".
type JoinConstraint struct {
	// join is the parent node
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
		return errorf("%T requires parent of type %s", jc, typeJoin)
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
	for _, child := range children {
		nodeCtx := child.Context()
		if _, ok := nodeCtx.(*antlr.TerminalNodeImpl); !ok {
			return errorf("expected leaf node, but got: %T", nodeCtx)
		}
	}

	if len(children) == 0 {
		jc.children = children
		return nil
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
