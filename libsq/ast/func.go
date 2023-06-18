package ast

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

var (
	_ Node         = (*FuncNode)(nil)
	_ ResultColumn = (*FuncNode)(nil)
)

// FuncNode models a function. For example, "COUNT()".
type FuncNode struct {
	baseNode
	fnName      string
	alias       string
	proprietary bool
}

// FuncName returns the function name.
func (fn *FuncNode) FuncName() string {
	return fn.fnName
}

// IsProprietary returns true if this is a DB-proprietary function, as
// opposed to a portable function. For example, SQLite has
// a "strftime" function. In the SLQ, this is referenced
// as "_strftime": SLQ uses the underscore to indicate a proprietary
// function.
func (fn *FuncNode) IsProprietary() bool {
	return fn.proprietary
}

// String returns a log/debug-friendly representation.
func (fn *FuncNode) String() string {
	str := nodeString(fn)
	if fn.alias != "" {
		str += ":" + fn.alias
	}
	return str
}

// Text implements ResultColumn.
func (fn *FuncNode) Text() string {
	return fn.ctx.GetText()
}

// Alias implements ResultColumn.
func (fn *FuncNode) Alias() string {
	return fn.alias
}

// SetChildren implements Node.
func (fn *FuncNode) SetChildren(children []Node) error {
	fn.setChildren(children)
	return nil
}

// AddChild implements Node.
func (fn *FuncNode) AddChild(child Node) error {
	// TODO: add check for valid FuncNode child types
	fn.addChild(child)
	return child.SetParent(fn)
}

// VisitFuncName implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFuncName(_ *slq.FuncNameContext) any {
	// no-op
	return nil
}

// VisitFuncElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFuncElement(ctx *slq.FuncElementContext) any {
	childCount := ctx.GetChildCount()
	if childCount == 0 || childCount > 2 {
		return errorf("parser: invalid function: expected 1 or 2 children, but got %d: %v",
			childCount, ctx.GetText())
	}

	// e.g. count(*)
	child1 := ctx.GetChild(0)
	fnCtx, ok := child1.(*slq.FuncContext)
	if !ok {
		return errorf("expected first child to be %T but was %T: %v", fnCtx, child1, ctx.GetText())
	}

	if err := v.VisitFunc(fnCtx); err != nil {
		return err
	}

	// Check if there's an alias
	if childCount == 2 {
		child2 := ctx.GetChild(1)
		aliasCtx, ok := child2.(*slq.AliasContext)
		if !ok {
			return errorf("expected second child to be %T but was %T: %v", aliasCtx, child2, ctx.GetText())
		}

		// VisitAlias will expect v.cur to be a FuncNode.
		lastNode := nodeLastChild(v.cur)
		fnNode, ok := lastNode.(*FuncNode)
		if !ok {
			return errorf("expected %T but got %T: %v", fnNode, lastNode, ctx.GetText())
		}

		return v.using(fnNode, func() any {
			return v.VisitAlias(aliasCtx)
		})
	}

	return nil
}

// VisitFunc implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFunc(ctx *slq.FuncContext) any {
	node := &FuncNode{fnName: ctx.FuncName().GetText()}
	if node.fnName[0] == '_' {
		node.fnName = node.fnName[1:]
	}

	node.ctx = ctx
	node.text = ctx.GetText()
	if err := node.SetParent(v.cur); err != nil {
		return err
	}

	if err := v.using(node, func() any {
		return v.VisitChildren(ctx)
	}); err != nil {
		return err
	}

	if node.alias == "" {
		node.alias = ctx.GetText()
		node.alias = strings.TrimPrefix(node.alias, "_")
	}

	return v.cur.AddChild(node)
}

// VisitCountFunc implements antlr.ParseTreeVisitor.
// Although the "count" func has special handling in the grammar (because
// it has a no-arg form, e.g. ".actor | count"), a regular FuncNode is
// inserted into the AST.
func (v *parseTreeVisitor) VisitCountFunc(ctx *slq.CountFuncContext) interface{} {
	node := &FuncNode{fnName: "count"}
	node.ctx = ctx
	node.text = ctx.GetText()

	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	if err := v.using(node, func() any {
		return v.VisitChildren(ctx)
	}); err != nil {
		return err
	}

	if len(node.Children()) == 0 && ctx.Alias() == nil {
		// If there's no children, and no alias, we explicitly set the
		// alias to "count".
		node.alias = "count"
	}

	return nil
}
