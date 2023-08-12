// Package ast holds types and functionality for the SLQ AST.
//
// The entrypoint is ast.Parse, which accepts a SLQ query, and
// returns an *ast.AST. The ast is a tree of ast.Node instances.
//
// Note that much of the testing of package ast is performed in
// package libsq.
package ast

import (
	"log/slog"
	"reflect"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// Parse parses the SLQ input string and builds the AST.
func Parse(log *slog.Logger, input string) (*AST, error) { //nolint:staticcheck
	// REVISIT: We need a better solution for disabling parser logging.
	log = lg.Discard() //nolint:staticcheck // Disable parser logging.
	ptree, err := parseSLQ(log, input)
	if err != nil {
		return nil, err
	}

	ast, err := buildAST(log, ptree)
	if err != nil {
		return nil, err
	}

	if err = verify(ast); err != nil {
		return nil, err
	}

	return ast, nil
}

// buildAST constructs sq's AST from a parse tree.
func buildAST(log *slog.Logger, query slq.IQueryContext) (*AST, error) {
	if query == nil {
		return nil, errorf("query is nil")
	}

	q, ok := query.(*slq.QueryContext)
	if !ok {
		return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	}

	tree := &parseTreeVisitor{log: log}

	if err := q.Accept(tree); err != nil {
		return nil, err.(error)
	}

	visitors := []struct {
		typ reflect.Type
		fn  nodeVisitorFn
	}{
		{typeSelectorNode, narrowTblSel},
		{typeSelectorNode, narrowTblColSel},
		{typeSelectorNode, narrowColSel},
		{typeRowRangeNode, verifyRowRange},
	}

	for _, visitor := range visitors {
		w := NewWalker(tree.ast).AddVisitor(visitor.typ, visitor.fn)
		if err := w.Walk(); err != nil {
			return nil, err
		}
	}

	return tree.ast, nil
}

// verify performs additional checks on the state of the built AST.
func verify(ast *AST) error {
	selCount := NewInspector(ast).CountNodes(typeSelectorNode)
	if selCount != 0 {
		return errorf("AST should have zero nodes of type %T but found %d",
			(*SelectorNode)(nil), selCount)
	}

	// TODO: Lots more checks could go here

	return nil
}

var _ Node = (*AST)(nil)

// AST is the Abstract Syntax Tree. It is the root node of a SQL query/stmt.
type AST struct {
	ctx  *slq.QueryContext
	segs []*SegmentNode
	text string
}

// Parent implements ast.Node.
func (a *AST) Parent() Node {
	return nil
}

// SetParent implements ast.Node.
func (a *AST) SetParent(parent Node) error {
	return errorf("root node (%T) cannot have parent: tried to add parent %T", a, parent)
}

// Children implements ast.Node.
func (a *AST) Children() []Node {
	nodes := make([]Node, len(a.segs))

	for i, seg := range a.segs {
		nodes[i] = seg
	}

	return nodes
}

// Segments returns the AST's segments (its direct children).
func (a *AST) Segments() []*SegmentNode {
	return a.segs
}

// AddChild implements ast.Node.
func (a *AST) AddChild(node Node) error {
	seg, ok := node.(*SegmentNode)
	if !ok {
		return errorf("expected *SegmentNode but got: %T", node)
	}

	a.AddSegment(seg)
	return nil
}

// SetChildren implements ast.Node.
func (a *AST) SetChildren(children []Node) error {
	segs := make([]*SegmentNode, len(children))

	for i, child := range children {
		seg, ok := child.(*SegmentNode)
		if !ok {
			return errorf("expected child of type %s, but got: %T", typeSegmentNode, child)
		}

		segs[i] = seg
	}

	a.segs = segs
	return nil
}

// context implements ast.Node.
func (a *AST) context() antlr.ParseTree {
	return a.ctx
}

// setContext implements ast.Node.
func (a *AST) setContext(ctx antlr.ParseTree) error {
	qCtx, ok := ctx.(*slq.QueryContext)
	if !ok {
		return errorf("expected *parser.QueryContext, but got %T", ctx)
	}

	a.ctx = qCtx
	return nil
}

// String implements ast.Node.
func (a *AST) String() string {
	return nodeString(a)
}

// Text implements ast.Node.
func (a *AST) Text() string {
	return a.ctx.GetText()
}

// AddSegment appends seg to the AST.
func (a *AST) AddSegment(seg *SegmentNode) {
	_ = seg.SetParent(a)
	a.segs = append(a.segs, seg)
}

// errorf builds an error. Error creation for this package
// was centralized here in the expectation that an AST-specific
// error type (annotated appropriately) would be returned.
func errorf(format string, v ...any) error {
	return errz.Errorf(format, v...)
}
