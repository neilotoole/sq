package ast

import (
	"fmt"

	"reflect"
	"strings"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/neilotoole/sq/libsq/slq"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// Segment models a segment of a query (the elements separated by pipes).
// For example, ".user | .uid, .username" is two segments (".user" and ".uid, .username").
type Segment struct {
	bn       BaseNode
	consumed bool
}

func (s *Segment) Parent() Node {
	return s.bn.Parent()
}

func (s *Segment) SetParent(parent Node) error {

	ast, ok := parent.(*AST)
	if !ok {
		return errorf("%T requires parent of type %s", s, TypeAST)
	}
	return s.bn.SetParent(ast)
}

func (s *Segment) Children() []Node {
	return s.bn.Children()
}

func (s *Segment) AddChild(child Node) error {
	s.bn.addChild(child)
	return child.SetParent(s)
}

func (s *Segment) SetChildren(children []Node) error {
	s.bn.setChildren(children)
	return nil
}

func (s *Segment) Context() antlr.ParseTree {
	return s.bn.Context()
}

func (s *Segment) SetContext(ctx antlr.ParseTree) error {

	segCtx, ok := ctx.(*slq.SegmentContext)
	if !ok {
		return errorf("expected *parser.SegmentContext, but got %T", ctx)
	}
	return s.bn.SetContext(segCtx)
}

// Unconsumed returns a slice of the segments elements that are not marked consumed.
//func (s *Segment) Unconsumed() []Element {
//
//	return nil
//}

// ChildType returns the expected Type of the segment's elements, based
// on the content of the segment's node's children. The type should be something
// like Selector|Fn
func (s *Segment) ChildType() (reflect.Type, error) {

	if len(s.Children()) == 0 {
		return nil, nil
	}
	_, err := s.HasCompatibleChildren()
	if err != nil {
		return nil, err
	}

	return reflect.TypeOf(s.Children()[0]), nil
}

// HasCompatibleNodes returns true if all the nodes of the segment are of a compatible type.
func (s *Segment) HasCompatibleChildren() (bool, error) {

	if len(s.Children()) == 0 {
		return true, nil
	}

	types := hashset.New()

	//fmt.Printf("segment[%d]: element count: %d\n", s.Index(), len(elems))
	for _, elem := range s.Children() {
		//fmt.Printf("  - elem [%d]: %T\n", i, elem)
		types.Add(reflect.TypeOf(elem))
	}

	if types.Size() > 1 {

		str := make([]string, types.Size())
		for i, typ := range types.Values() {
			str[i] = fmt.Sprintf("%s", typ)
		}

		return false, fmt.Errorf("segment [%d] has more than one element node type: [%s]", s.SegIndex(), strings.Join(str, ", "))
	}

	return true, nil
}

// Index returns the index of this segment within the IR.
func (s *Segment) SegIndex() int {

	for i, seg := range s.bn.parent.Children() {
		if s == seg {
			return i
		}
	}

	return -1
}

func (s *Segment) String() string {

	if len(s.Children()) == 1 {
		return fmt.Sprintf("segment[%d]: [1 element]", s.SegIndex())
	}

	return fmt.Sprintf("segment[%d]: [%d elements]", s.SegIndex(), len(s.Children()))
}

func (s *Segment) Text() string {
	return s.bn.Context().GetText()
}

// Prev returns the previous segment, or nil.
func (s *Segment) Prev() *Segment {

	parent := s.Parent()
	children := parent.Children()
	index := -1

	for i, child := range children {

		childSeg, ok := child.(*Segment)
		if !ok {
			// should never happen
			panic("sibling is not *ast.Segment")
		}

		if childSeg == s {
			index = i
			break
		}
	}

	if index == -1 {
		panic(fmt.Sprintf("did not find index for this segment: %s", s))
	}

	if index == 0 {
		return nil
	}

	return children[index-1].(*Segment)
}

// Next returns the next segment, or nil.
func (s *Segment) Next() *Segment {

	for i, seg := range s.bn.parent.Children() {
		if seg == s {
			if i >= len(s.bn.parent.Children())-1 {
				return nil
			}

			return s.bn.parent.Children()[i+1].(*Segment)
		}
	}

	return nil
}
