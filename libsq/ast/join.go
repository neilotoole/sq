package ast

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

// VisitJoin implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoin(ctx *slq.JoinContext) any {
	// parent node must be a segment
	seg, ok := v.cur.(*SegmentNode)
	if !ok {
		return errorf("parent of JOIN() must be SegmentNode, but got: %T", v.cur)
	}

	join := &JoinNode{seg: seg, ctx: ctx}
	err := seg.AddChild(join)
	if err != nil {
		return err
	}

	expr := ctx.JoinConstraint()
	if expr == nil {
		return nil
	}

	// the join contains a constraint, let's hit it
	v.cur = join
	err2 := v.VisitJoinConstraint(expr.(*slq.JoinConstraintContext))
	if err2 != nil {
		return err2
	}
	// set cur back to previous
	v.cur = seg
	return nil
}

// VisitJoinConstraint implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoinConstraint(ctx *slq.JoinConstraintContext) any {
	joinNode, ok := v.cur.(*JoinNode)
	if !ok {
		return errorf("JOIN constraint must have JOIN parent, but got %T", v.cur)
	}

	// the constraint could be empty
	children := ctx.GetChildren()
	if len(children) == 0 {
		return nil
	}

	// the constraint could be a single SEL (in which case, there's no comparison operator)
	if ctx.Cmpr() == nil {
		// there should be exactly one SEL
		sels := ctx.AllSelector()
		if len(sels) != 1 {
			return errorf("JOIN constraint without a comparison operator must have exactly one selector")
		}

		joinExprNode := &JoinConstraint{join: joinNode, ctx: ctx}

		colSelNode, err := newSelectorNode(joinExprNode, sels[0])
		if err != nil {
			return err
		}

		if err := joinExprNode.AddChild(colSelNode); err != nil {
			return err
		}

		return joinNode.AddChild(joinExprNode)
	}

	// We've got a comparison operator
	sels := ctx.AllSelector()
	if len(sels) != 2 {
		// REVISIT: probably unnecessary, should be caught by the parser
		return errorf("JOIN constraint must have 2 operands (left & right), but got %d", len(sels))
	}

	join, ok := v.cur.(*JoinNode)
	if !ok {
		return errorf("JoinConstraint must have JoinNode parent, but got %T", v.cur)
	}
	joinCondition := &JoinConstraint{join: join, ctx: ctx}

	leftSel, err := newSelectorNode(joinCondition, sels[0])
	if err != nil {
		return err
	}

	if err = joinCondition.AddChild(leftSel); err != nil {
		return err
	}

	cmpr := newCmpr(joinCondition, ctx.Cmpr())
	if err = joinCondition.AddChild(cmpr); err != nil {
		return err
	}

	rightSel, err := newSelectorNode(joinCondition, sels[1])
	if err != nil {
		return err
	}

	if err = joinCondition.AddChild(rightSel); err != nil {
		return err
	}

	return join.AddChild(joinCondition)
}

var _ Node = (*JoinNode)(nil)

// JoinNode models a SQL JOIN node. It has a child of type JoinConstraint.
type JoinNode struct {
	seg        *SegmentNode
	ctx        antlr.ParseTree
	constraint *JoinConstraint
	leftTbl    *TblSelectorNode
	rightTbl   *TblSelectorNode
}

// LeftTbl is the selector for the left table of the join.
func (jn *JoinNode) LeftTbl() *TblSelectorNode {
	return jn.leftTbl
}

// RightTbl is the selector for the right table of the join.
func (jn *JoinNode) RightTbl() *TblSelectorNode {
	return jn.rightTbl
}

// Tabler implements the Tabler marker interface.
func (jn *JoinNode) tabler() {
	// no-op
}

func (jn *JoinNode) Parent() Node {
	return jn.seg
}

func (jn *JoinNode) SetParent(parent Node) error {
	seg, ok := parent.(*SegmentNode)
	if !ok {
		return errorf("%T requires parent of type %s", jn, typeSegmentNode)
	}
	jn.seg = seg
	return nil
}

func (jn *JoinNode) Children() []Node {
	if jn.constraint == nil {
		return []Node{}
	}

	return []Node{jn.constraint}
}

func (jn *JoinNode) AddChild(node Node) error {
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

func (jn *JoinNode) SetChildren(children []Node) error {
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

func (jn *JoinNode) Context() antlr.ParseTree {
	return jn.ctx
}

func (jn *JoinNode) SetContext(ctx antlr.ParseTree) error {
	jn.ctx = ctx
	return nil
}

func (jn *JoinNode) Text() string {
	return jn.ctx.GetText()
}

func (jn *JoinNode) Segment() *SegmentNode {
	return jn.seg
}

func (jn *JoinNode) String() string {
	text := nodeString(jn)

	leftTblName := ""
	rightTblName := ""

	if jn.leftTbl != nil {
		leftTblName, _ = jn.leftTbl.SelValue()
	}
	if jn.rightTbl != nil {
		rightTblName, _ = jn.rightTbl.SelValue()
	}

	text += fmt.Sprintf(" |  left_table: {%s}  |  right_table: {%s}", leftTblName, rightTblName)
	return text
}

var _ Node = (*JoinConstraint)(nil)

// JoinConstraint models a join's constraint.
// For example the elements inside the parentheses
// in "join(.uid == .user_id)".
type JoinConstraint struct {
	// join is the parent node
	join     *JoinNode
	ctx      antlr.ParseTree
	children []Node
}

func (n *JoinConstraint) Parent() Node {
	return n.join
}

func (n *JoinConstraint) SetParent(parent Node) error {
	join, ok := parent.(*JoinNode)
	if !ok {
		return errorf("%T requires parent of type %s", n, typeJoinNode)
	}
	n.join = join
	return nil
}

func (n *JoinConstraint) Children() []Node {
	return n.children
}

func (n *JoinConstraint) AddChild(child Node) error {
	nodeCtx := child.Context()

	switch nodeCtx.(type) {
	case *antlr.TerminalNodeImpl:
	case *slq.SelectorContext:
	default:
		return errorf("cannot add child node %T to %T", nodeCtx, n.ctx)
	}

	n.children = append(n.children, child)
	return nil
}

func (n *JoinConstraint) SetChildren(children []Node) error {
	for _, child := range children {
		nodeCtx := child.Context()

		switch nodeCtx.(type) {
		case *antlr.TerminalNodeImpl:
		case *slq.SelectorContext:
		default:
			return errorf("cannot add child node %T to %T", nodeCtx, n.ctx)
		}
	}

	if len(children) == 0 {
		n.children = children
		return nil
	}

	n.children = children
	return nil
}

func (n *JoinConstraint) Context() antlr.ParseTree {
	return n.ctx
}

func (n *JoinConstraint) SetContext(ctx antlr.ParseTree) error {
	n.ctx = ctx // TODO: check for correct type
	return nil
}

func (n *JoinConstraint) Text() string {
	return n.ctx.GetText()
}

func (n *JoinConstraint) String() string {
	return nodeString(n)
}
