package ast

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

// VisitJoin implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoin(ctx *slq.JoinContext) any {
	// parent node must be a segment
	seg, ok := v.cur.(*SegmentNode)
	if !ok {
		return errorf("parent of JOIN() must be SegmentNode, but got: %T", v.cur)
	}

	var err error
	node := &JoinNode{
		seg:  seg,
		ctx:  ctx,
		text: ctx.GetText(),
	}

	if node.typ, err = getJoinType(ctx); err != nil {
		return err
	}

	if ctx.JoinTable() == nil {
		return errz.Errorf("invalid join: table is nil")
	}

	var jtCtx *slq.JoinTableContext
	if jtCtx, ok = ctx.JoinTable().(*slq.JoinTableContext); !ok {
		return errz.Errorf("invalid join: table: expected %T but got %T", jtCtx, ctx.JoinTable())
	}

	if e := v.using(node, func() any {
		return v.VisitJoinTable(jtCtx)
	}); e != nil {
		return e
	}

	if ctx.Expr() != nil {
		// Expression can be nil for cross join, etc.
		var exprCtx *slq.ExprContext
		if exprCtx, ok = ctx.Expr().(*slq.ExprContext); !ok {
			return errz.Errorf("invalid join: expression: expected %T but got %T", exprCtx, ctx.Expr())
		}

		if e := v.using(node, func() any {
			return v.VisitExpr(exprCtx)
		}); e != nil {
			return e
		}
	}

	return seg.AddChild(node)
}

// VisitJoinTable implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoinTable(ctx *slq.JoinTableContext) any {
	joinNode, ok := v.cur.(*JoinNode)
	if !ok {
		return errorf("JOIN constraint must have JOIN parent, but got %T", v.cur)
	}

	var handle string
	var tblName string

	if ctx.HANDLE() != nil {
		// It's ok to have a nil/empty handle
		handle = ctx.HANDLE().GetText()
	}

	if ctx.NAME() == nil {
		return errorf("invalid %T: table name is nil", ctx)
	}

	tblName, err := extractSelVal(ctx.NAME())
	if err != nil {
		return err
	}

	tblSelNode := &TblSelectorNode{
		SelectorNode: SelectorNode{
			baseNode: baseNode{
				parent: joinNode,
				ctx:    ctx.NAME(),
				text:   ctx.NAME().GetText(),
			},
			name0: tblName,
		},
		handle:  handle,
		tblName: tblName,
	}

	var aliasCtx *slq.AliasContext
	if ctx.Alias() != nil {
		if aliasCtx, ok = ctx.Alias().(*slq.AliasContext); !ok {
			return errorf("invalid %T: expected %T but got %T", ctx, aliasCtx, ctx.Alias())
		}
	}

	if e := v.using(tblSelNode, func() any {
		return v.VisitAlias(aliasCtx)
	}); e != nil {
		return e
	}

	joinNode.rightTbl = tblSelNode
	return nil

	//v.VisitAlias()
	//
	//ctx.HANDLE()
	//
	//// the constraint could be empty
	//children := ctx.GetChildren()
	//if len(children) == 0 {
	//	return nil
	//}
	//
	//// the constraint could be a single SEL (in which case, there's no comparison operator)
	//if ctx.Cmpr() == nil {
	//	// there should be exactly one SEL
	//	sels := ctx.AllSelector()
	//	if len(sels) != 1 {
	//		return errorf("JOIN constraint without a comparison operator must have exactly one selector")
	//	}
	//
	//	joinExprNode := &JoinConstraint{join: joinNode, ctx: ctx}
	//
	//	colSelNode, err := newSelectorNode(joinExprNode, sels[0])
	//	if err != nil {
	//		return err
	//	}
	//
	//	if err := joinExprNode.AddChild(colSelNode); err != nil {
	//		return err
	//	}
	//
	//	return joinNode.AddChild(joinExprNode)
	//}
	//
	//// We've got a comparison operator
	//sels := ctx.AllSelector()
	//if len(sels) != 2 {
	//	// REVISIT: probably unnecessary, should be caught by the parser
	//	return errorf("JOIN constraint must have 2 operands (left & right), but got %d", len(sels))
	//}
	//
	//join, ok := v.cur.(*JoinNode)
	//if !ok {
	//	return errorf("JoinConstraint must have JoinNode parent, but got %T", v.cur)
	//}
	//joinCondition := &JoinConstraint{join: join, ctx: ctx}
	//
	//leftSel, err := newSelectorNode(joinCondition, sels[0])
	//if err != nil {
	//	return err
	//}
	//
	//if err = joinCondition.AddChild(leftSel); err != nil {
	//	return err
	//}
	//
	//cmpr := newCmprNode(joinCondition, ctx.Cmpr())
	//if err = joinCondition.AddChild(cmpr); err != nil {
	//	return err
	//}
	//
	//rightSel, err := newSelectorNode(joinCondition, sels[1])
	//if err != nil {
	//	return err
	//}
	//
	//if err = joinCondition.AddChild(rightSel); err != nil {
	//	return err
	//}
	//
	//return join.AddChild(joinCondition)
}

var _ Node = (*JoinNode)(nil)

// JoinNode models a SQL JOIN node.
type JoinNode struct {
	seg            *SegmentNode
	ctx            antlr.ParseTree
	text           string
	typ            JoinType
	constraintExpr *ExprNode

	// FIXME: rename rightTbl to targetTbl
	rightTbl *TblSelectorNode
}

// Constraint returns the join constraint, which
// may be nil.
func (n *JoinNode) Constraint() *ExprNode {
	return n.constraintExpr
}

// JoinType returns the join type.
func (n *JoinNode) JoinType() JoinType {
	return n.typ
}

// RightTbl is the selector for the right table of the join.
func (n *JoinNode) RightTbl() *TblSelectorNode {
	return n.rightTbl
}

//// Tabler implements the ast.Tabler marker interface.
//func (n *JoinNode) tabler() {
//	// no-op
//}

// Parent implements ast.Node.
func (n *JoinNode) Parent() Node {
	return n.seg
}

// SetParent implements ast.Node.
func (n *JoinNode) SetParent(parent Node) error {
	seg, ok := parent.(*SegmentNode)
	if !ok {
		return errorf("%T requires parent of type %s", n, typeSegmentNode)
	}
	n.seg = seg
	return nil
}

// Children implements ast.Node.
func (n *JoinNode) Children() []Node {
	if n.constraintExpr == nil {
		return []Node{}
	}

	return []Node{n.constraintExpr}
}

// AddChild implements ast.Node.
func (n *JoinNode) AddChild(node Node) error {
	expr, ok := node.(*ExprNode)
	if !ok {
		return errorf("join child must be %T, but got: %T", expr, node)
	}

	if n.constraintExpr != nil {
		return errorf("JOIN() has max 1 child: failed to add: %T", node)
	}

	n.constraintExpr = expr
	return nil
}

// SetChildren implements ast.Node.
func (n *JoinNode) SetChildren(children []Node) error {
	switch len(children) {
	case 0:
		n.constraintExpr = nil
		return nil
	case 1:
		n.constraintExpr = nil
		return n.AddChild(children[0])
	default:
		return errorf("join: max of one child allowed; failed to add %d children", len(children))
	}
}

// context implements ast.Node.
func (n *JoinNode) context() antlr.ParseTree {
	return n.ctx
}

// setContext implements ast.Node.
func (n *JoinNode) setContext(ctx antlr.ParseTree) error {
	n.ctx = ctx
	return nil
}

// Text implements ast.Node.
func (n *JoinNode) Text() string {
	return n.ctx.GetText()
}

func (n *JoinNode) Segment() *SegmentNode {
	return n.seg
}

// String implements ast.Node.
func (n *JoinNode) String() string {
	text := nodeString(n)

	rightTblName := ""
	if n.rightTbl != nil {
		rightTblName, _ = n.rightTbl.SelValue()
	}

	text += fmt.Sprintf("|target:%s", rightTblName)
	return text
}

//
//var _ Node = (*JoinConstraint)(nil)
//
//// JoinConstraint models a join's constraint.
//// For example the elements inside the parentheses
//// in "join(.uid == .user_id)".
//type JoinConstraint struct {
//	// join is the parent node
//	join     *JoinNode
//	ctx      antlr.ParseTree
//	children []Node
//}
//
//func (n *JoinConstraint) Parent() Node {
//	return n.join
//}
//
//func (n *JoinConstraint) SetParent(parent Node) error {
//	join, ok := parent.(*JoinNode)
//	if !ok {
//		return errorf("%T requires parent of type %s", n, typeJoinNode)
//	}
//	n.join = join
//	return nil
//}
//
//func (n *JoinConstraint) Children() []Node {
//	return n.children
//}
//
//func (n *JoinConstraint) AddChild(child Node) error {
//	nodeCtx := child.Context()
//
//	switch nodeCtx.(type) {
//	case *antlr.TerminalNodeImpl:
//	case *slq.SelectorContext:
//	default:
//		return errorf("cannot add child node %T to %T", nodeCtx, n.ctx)
//	}
//
//	n.children = append(n.children, child)
//	return nil
//}
//
//func (n *JoinConstraint) SetChildren(children []Node) error {
//	for _, child := range children {
//		nodeCtx := child.Context()
//
//		switch nodeCtx.(type) {
//		case *antlr.TerminalNodeImpl:
//		case *slq.SelectorContext:
//		default:
//			return errorf("cannot add child node %T to %T", nodeCtx, n.ctx)
//		}
//	}
//
//	if len(children) == 0 {
//		n.children = children
//		return nil
//	}
//
//	n.children = children
//	return nil
//}
//
//func (n *JoinConstraint) Context() antlr.ParseTree {
//	return n.ctx
//}
//
//func (n *JoinConstraint) setContext(ctx antlr.ParseTree) error {
//	n.ctx = ctx // TODO: check for correct type
//	return nil
//}
//
//func (n *JoinConstraint) Text() string {
//	return n.ctx.GetText()
//}
//
//func (n *JoinConstraint) String() string {
//	return nodeString(n)
//}

// JoinType indicates the type of join, e.g. "INNER JOIN"
// or "RIGHT OUTER JOIN", etc.
type JoinType string

const (
	Join           JoinType = "join"
	JoinInner      JoinType = "inner_join"
	JoinLeft       JoinType = "left_join"
	JoinLeftOuter  JoinType = "left_outer_join"
	JoinRight      JoinType = "right_join"
	JoinRightOuter JoinType = "right_outer_join"
	JoinFullOuter  JoinType = "full_outer_join"
	JoinCross      JoinType = "cross_join"
)

func getJoinType(ctx *slq.JoinContext) (JoinType, error) {
	if ctx == nil {
		return "", errz.Errorf("%T is nil", ctx)
	}

	jt := ctx.JOIN_TYPE()
	if jt == nil {
		return "", errz.Errorf("JOIN_TYPE (%T) is nil", jt)
	}

	text := jt.GetText()
	switch text {
	case string(Join):
		return Join, nil
	case string(JoinInner):
		return JoinInner, nil
	case string(JoinLeft), "ljoin":
		return JoinLeft, nil
	case string(JoinLeftOuter), "lojoin":
		return JoinLeftOuter, nil
	case string(JoinRight), "rjoin":
		return JoinRight, nil
	case string(JoinRightOuter), "rojoin":
		return JoinRightOuter, nil
	case string(JoinFullOuter), "fojoin":
		return JoinFullOuter, nil
	case string(JoinCross), "cjoin":
		return JoinCross, nil
	default:
		return "", errz.Errorf("invalid join type {%s}", text)
	}
}
