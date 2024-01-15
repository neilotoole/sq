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
	node.text = ctx.GetText()

	node.handle = ctx.HANDLE().GetText()

	var err error
	selTbl, err := extractSelVal(ctx.NAME())
	if err != nil {
		return err
	}
	node.tbl = tablefq.From(selTbl)

	return v.cur.AddChild(node)
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
