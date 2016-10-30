package ast

import (
	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/slq"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// AST is sq's Abstract Syntax Tree.
type AST struct {
	ctx        *slq.QueryContext
	Datasource string
	segs       []*Segment
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
			return errorf("expected child of type %s, but got: %T", TypeSegment, child)
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
	seg.SetParent(a)
	a.segs = append(a.segs, seg)
}

// Error is an error generated within the ast package.
type Error struct {
	msg string
}

func (e *Error) Error() string {
	return e.msg
}

func errorf(format string, v ...interface{}) *Error {
	err := &Error{msg: fmt.Sprintf(format, v...)}
	lg.Depth(1).Warnf("error created: %s", err.msg)
	return err
}
