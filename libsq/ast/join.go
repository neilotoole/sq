package ast

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/core/tablefq"

	"github.com/neilotoole/sq/libsq/core/jointype"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/antlr4-go/antlr/v4"
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

	if node.jt, node.jtVal, err = getJoinType(ctx); err != nil {
		return err
	}

	if ctx.JoinTable() == nil {
		return errz.Errorf("invalid join: %s: table is nil: %s", node.jtVal, node.text)
	}

	var jtCtx *slq.JoinTableContext
	if jtCtx, ok = ctx.JoinTable().(*slq.JoinTableContext); !ok {
		return errz.Errorf("invalid join: %s: invalid table type: expected %T but got %T: %s",
			node.jtVal, jtCtx, ctx.JoinTable(), node.text)
	}

	if e := v.using(node, func() any {
		return v.VisitJoinTable(jtCtx)
	}); e != nil {
		return e
	}

	if ctx.Expr() == nil {
		if node.jt.HasPredicate() {
			return errorf("invalid join: %s: predicate required: %s",
				node.jtVal, node.text)
		}
	}

	if ctx.Expr() != nil {
		// Expression can be nil for cross join.
		var exprCtx *slq.ExprContext
		if exprCtx, ok = ctx.Expr().(*slq.ExprContext); !ok {
			return errorf("invalid join: %s: expression type: expected %T but got %T: %s",
				node.jtVal, exprCtx, ctx.Expr(), node.text)
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
		handle: handle,
		tbl:    tablefq.From(tblName),
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

	joinNode.targetTbl = tblSelNode
	return nil
}

var _ Node = (*JoinNode)(nil)

// JoinNode models a SQL JOIN node.
type JoinNode struct {
	seg           *SegmentNode
	ctx           antlr.ParseTree
	text          string
	jt            jointype.Type
	jtVal         string
	predicateExpr *ExprNode

	targetTbl *TblSelectorNode
}

// Predicate returns the join predicate, which
// may be nil.
func (n *JoinNode) Predicate() *ExprNode {
	return n.predicateExpr
}

// JoinType returns the join type.
func (n *JoinNode) JoinType() jointype.Type {
	return n.jt
}

// Table is the selector for join's target table.
func (n *JoinNode) Table() *TblSelectorNode {
	return n.targetTbl
}

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
	if n.predicateExpr == nil {
		return []Node{}
	}

	return []Node{n.predicateExpr}
}

// AddChild implements ast.Node.
func (n *JoinNode) AddChild(node Node) error {
	expr, ok := node.(*ExprNode)
	if !ok {
		return errorf("join child must be %T, but got: %T", expr, node)
	}

	if n.predicateExpr != nil {
		return errorf("JOIN() has max 1 child: failed to add: %T", node)
	}

	n.predicateExpr = expr
	return nil
}

// SetChildren implements ast.Node.
func (n *JoinNode) SetChildren(children []Node) error {
	switch len(children) {
	case 0:
		n.predicateExpr = nil
		return nil
	case 1:
		n.predicateExpr = nil
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
	if n.targetTbl != nil {
		rightTblName, _ = n.targetTbl.SelValue()
	}

	text += fmt.Sprintf("|target:%s", rightTblName)
	return text
}

// getJoinType returns the canonical join type, as well as the
// input value (which could be the canonical type, or the type's alias).
func getJoinType(ctx *slq.JoinContext) (typ jointype.Type, val string, err error) {
	if ctx == nil {
		return "", val, errorf("%T is nil", ctx)
	}

	terminal := ctx.JOIN_TYPE()
	if terminal == nil {
		// Shouldn't happen
		return "", val, errz.Errorf("JOIN_TYPE (%T) is nil", terminal)
	}

	val = terminal.GetText()
	err = typ.UnmarshalText([]byte(val))
	return typ, val, err
}
