package ast

import (
	"slices"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

// HandleNode models a source handle such as "@sakila".
type HandleNode struct {
	baseNode
}

// Handle returns the handle value, e.g. "@sakila".
func (d *HandleNode) Handle() string {
	return d.text
}

// String returns a log/debug-friendly representation.
func (d *HandleNode) String() string {
	return nodeString(d)
}

// VisitHandle implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHandle(ctx *slq.HandleContext) any {
	node := &HandleNode{}
	node.text = ctx.GetText()
	node.parent = v.cur
	node.ctx = ctx.HANDLE()
	return v.cur.AddChild(node)
}

// VisitHandleTable implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHandleTable(ctx *slq.HandleTableContext) any {
	node := &TblSelectorNode{}
	node.parent = v.cur
	node.ctx = ctx
	// ctx.GetText() now includes any trailing ":alias"; keep node.text to the
	// handle+table portion (HANDLE + NAME, where NAME carries its leading dot)
	// so debug/inspector output stays clean.
	node.text = ctx.HANDLE().GetText() + ctx.NAME().GetText()

	node.handle = ctx.HANDLE().GetText()

	selTbl, err := extractSelVal(ctx.NAME())
	if err != nil {
		return err
	}
	node.tbl = tablefq.From(selTbl)

	if err = v.cur.AddChild(node); err != nil {
		return err
	}

	// Optional trailing alias, e.g. @sakila.actor:a. Mirror VisitJoinTable:
	// make the new node current and visit the alias, which sets node.alias
	// (this path also handles reserved-word and quoted aliases).
	if ctx.Alias() != nil {
		aliasCtx, ok := ctx.Alias().(*slq.AliasContext)
		if !ok {
			return errorf("invalid %T: expected %T but got %T", ctx, aliasCtx, ctx.Alias())
		}
		if e := v.using(node, func() any { return v.VisitAlias(aliasCtx) }); e != nil {
			return e
		}
	}

	return nil
}

// ExtractHandles returns a sorted slice of all handles mentioned
// in the AST. Duplicate mentions are removed.
func ExtractHandles(ast *AST) []string {
	var handles []string
	handleNodes := FindNodes[*HandleNode](ast)
	for _, n := range handleNodes {
		handles = append(handles, n.Handle())
	}

	joinNodes := FindNodes[*JoinNode](ast)
	for _, n := range joinNodes {
		if n != nil && n.Table().Handle() != "" {
			handles = append(handles, n.Table().Handle())
		}
	}

	tblSelNodes := FindNodes[*TblSelectorNode](ast)
	for _, n := range tblSelNodes {
		if n.Handle() != "" {
			handles = append(handles, n.Handle())
		}
	}

	handles = lo.Uniq(handles)
	slices.Sort(handles)
	return handles
}
