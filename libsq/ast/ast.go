// Package ast holds types and functionality for the SLQ AST.
//
// Note: the SLQ language implementation is fairly rudimentary
// and has some incomplete functionality.
package ast

import (
	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// Parse parses the SLQ input string and builds the AST.
func Parse(log lg.Log, input string) (*AST, error) {
	ptree, err := parseSLQ(log, input)
	if err != nil {
		return nil, err
	}

	atree, err := buildAST(log, ptree)
	if err != nil {
		return nil, err
	}

	return atree, nil
}

// buildAST constructs sq's AST from a parse tree.
func buildAST(log lg.Log, query slq.IQueryContext) (*AST, error) {
	if query == nil {
		return nil, errorf("query is nil")
	}

	q, ok := query.(*slq.QueryContext)
	if !ok {
		return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	}

	v := &parseTreeVisitor{log: lg.Discard()}
	er := q.Accept(v)
	if er != nil {
		return nil, er.(error)
	}

	err := NewWalker(log, v.AST).AddVisitor(typeSelector, narrowTblSel).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(log, v.AST).AddVisitor(typeSelector, narrowColSel).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(log, v.AST).AddVisitor(typeJoin, determineJoinTables).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(log, v.AST).AddVisitor(typeRowRange, visitCheckRowRange).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(log, v.AST).AddVisitor(typeExpr, findWhereClause).Walk()
	if err != nil {
		return nil, err
	}

	return v.AST, nil
}

var _ Node = (*AST)(nil)

// AST is the Abstract Syntax Tree. It is the root node of a SQL query/stmt.
type AST struct {
	ctx  *slq.QueryContext
	segs []*Segment
}

func (a *AST) Parent() Node {
	return nil
}

func (a *AST) SetParent(parent Node) error {
	return errorf("root node (%T) cannot have parent: tried to add parent %T", a, parent)
}

func (a *AST) Children() []Node {
	nodes := make([]Node, len(a.segs))

	for i, seg := range a.segs {
		nodes[i] = seg
	}

	return nodes
}

func (a *AST) Segments() []*Segment {
	return a.segs
}

func (a *AST) AddChild(node Node) error {
	seg, ok := node.(*Segment)
	if !ok {
		return errorf("expected *Segment but got: %T", node)
	}

	a.AddSegment(seg)
	return nil
}

func (a *AST) SetChildren(children []Node) error {
	segs := make([]*Segment, len(children))

	for i, child := range children {
		seg, ok := child.(*Segment)
		if !ok {
			return errorf("expected child of type %s, but got: %T", typeSegment, child)
		}

		segs[i] = seg
	}

	a.segs = segs
	return nil
}

func (a *AST) Context() antlr.ParseTree {
	return a.ctx
}

func (a *AST) SetContext(ctx antlr.ParseTree) error {
	qCtx, ok := ctx.(*slq.QueryContext)
	if !ok {
		return errorf("expected *parser.QueryContext, but got %T", ctx)
	}

	a.ctx = qCtx
	return nil
}

func (a *AST) String() string {
	return nodeString(a)
}

func (a *AST) Text() string {
	return a.ctx.GetText()
}

// AddSegment appends seg to the AST.
func (a *AST) AddSegment(seg *Segment) {
	_ = seg.SetParent(a)
	a.segs = append(a.segs, seg)
}

// errorf builds an error. Error creation for this package
// was centralized here in the expectation that an AST-specific
// error type (annotated appropriately) would be returned.
func errorf(format string, v ...interface{}) error {
	return errz.Errorf(format, v...)
}
