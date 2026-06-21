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
	// Anchor ctx/text on the NAME token (the table), not the whole
	// handleTable: NAME excludes both the handle and any trailing ":alias",
	// so Node.Text() reports just the table reference. This mirrors
	// VisitJoinTable. The handle is still reported via Handle()/String().
	node.ctx = ctx.NAME()
	node.text = ctx.NAME().GetText()

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
	// make the new node current and visit the alias, which sets node.alias.
	// VisitAlias handles every alias shape (ID, quoted STRING, and reserved
	// words such as :count); an argument reference (e.g. :$x) is rejected.
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
