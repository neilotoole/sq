package ast

import (
	"fmt"
	"reflect"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// VisitSegment implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSegment(ctx *slq.SegmentContext) any {
	seg := newSegmentNode(v.ast, ctx)
	v.ast.AddSegment(seg)
	v.cur = seg

	return v.VisitChildren(ctx)
}

func newSegmentNode(ast *AST, ctx *slq.SegmentContext) *SegmentNode {
	seg := &SegmentNode{}
	seg.bn.ctx = ctx
	seg.bn.parent = ast
	seg.bn.text = ctx.GetText()
	return seg
}

var _ Node = (*SegmentNode)(nil)

// SegmentNode models a segment of a query (the elements separated by pipes).
// For example, ".user | .uid, .username" is two segments: ".user",
// and ".uid, .username".
type SegmentNode struct {
	bn baseNode
}

// Parent implements ast.Node.
func (s *SegmentNode) Parent() Node {
	return s.bn.Parent()
}

// SetParent implements ast.Node.
func (s *SegmentNode) SetParent(parent Node) error {
	ast, ok := parent.(*AST)
	if !ok {
		return errorf("%T requires parent of type %s", s, typeAST)
	}
	return s.bn.SetParent(ast)
}

// Children implements ast.Node.
func (s *SegmentNode) Children() []Node {
	return s.bn.Children()
}

// AddChild implements ast.Node.
func (s *SegmentNode) AddChild(child Node) error {
	s.bn.addChild(child)
	return child.SetParent(s)
}

// SetChildren implements ast.Node.
func (s *SegmentNode) SetChildren(children []Node) error {
	s.bn.doSetChildren(children)
	return nil
}

// context implements ast.Node.
func (s *SegmentNode) context() antlr.ParseTree {
	return s.bn.context()
}

// setContext implements ast.Node.
func (s *SegmentNode) setContext(ctx antlr.ParseTree) error {
	segCtx, ok := ctx.(*slq.SegmentContext)
	if !ok {
		return errorf("expected *parser.SegmentContext, but got %T", ctx)
	}
	return s.bn.setContext(segCtx)
}

// ChildType returns the expected Type of the segment's elements, based
// on the content of the segment's node's children. The type should be something
// like SelectorNode|FuncNode.
func (s *SegmentNode) ChildType() (reflect.Type, error) {
	if len(s.Children()) == 0 {
		return nil, nil
	}
	_, err := s.uniformChildren()
	if err != nil {
		return nil, err
	}

	return reflect.TypeOf(s.Children()[0]), nil
}

// uniformChildren returns true if all the nodes of the segment
// are of a uniform type.
func (s *SegmentNode) uniformChildren() (bool, error) {
	if len(s.Children()) == 0 {
		return true, nil
	}

	typs := map[string]struct{}{}
	for _, elem := range s.Children() {
		typs[reflect.TypeOf(elem).String()] = struct{}{}
	}

	if len(typs) > 1 {
		var str []string
		for typ := range typs {
			str = append(str, typ)
		}

		return false, fmt.Errorf("segment [%d] has more than one element node type: [%s]", s.SegIndex(),
			strings.Join(str, ", "))
	}

	return true, nil
}

// SegIndex returns the index of this segment.
func (s *SegmentNode) SegIndex() int {
	for i, seg := range s.bn.parent.Children() {
		if s == seg {
			return i
		}
	}

	return -1
}

// String returns a log/debug-friendly representation.
func (s *SegmentNode) String() string {
	if len(s.Children()) == 1 {
		return fmt.Sprintf("segment[%d]: [1 element]", s.SegIndex())
	}

	return fmt.Sprintf("segment[%d]: [%d elements]", s.SegIndex(), len(s.Children()))
}

// Text implements ast.Node.
func (s *SegmentNode) Text() string {
	return s.bn.context().GetText()
}

// Prev returns the previous segment, or nil if this is
// the first segment.
func (s *SegmentNode) Prev() *SegmentNode {
	parent := s.Parent()
	children := parent.Children()
	index := -1

	for i, child := range children {
		childSeg, ok := child.(*SegmentNode)
		if !ok {
			// should never happen
			panic("sibling is not *ast.SegmentNode")
		}

		if childSeg == s {
			index = i
			break
		}
	}

	if index == -1 {
		// Should never happen
		panic(fmt.Sprintf("did not find index for this segment: %s", s))
	}

	if index == 0 {
		return nil
	}

	return children[index-1].(*SegmentNode)
}

// Next returns the next segment, or nil if this is the last segment.
func (s *SegmentNode) Next() *SegmentNode {
	for i, seg := range s.bn.parent.Children() {
		if seg == s {
			if i >= len(s.bn.parent.Children())-1 {
				return nil
			}

			return s.bn.parent.Children()[i+1].(*SegmentNode)
		}
	}

	return nil
}
