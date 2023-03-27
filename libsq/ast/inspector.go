package ast

import (
	"reflect"

	"github.com/samber/lo"

	"github.com/ryboe/q"

	"github.com/neilotoole/lg"
)

// Inspector provides functionality for AST interrogation.
type Inspector struct {
	log lg.Log
	ast *AST
}

// NewInspector returns an Inspector instance for ast.
func NewInspector(log lg.Log, ast *AST) *Inspector {
	return &Inspector{log: log, ast: ast}
}

// CountNodes counts the number of nodes having typ.
func (in *Inspector) CountNodes(typ reflect.Type) int {
	count := 0
	w := NewWalker(in.log, in.ast)
	w.AddVisitor(typ, func(log lg.Log, w *Walker, node Node) error {
		count++
		if typ == typeSelectorNode {
			// found it
			// FIXME: delete this
			q.Q("found it", node)
		}
		return nil
	})

	_ = w.Walk()
	return count
}

// FindNodes returns the nodes having typ.
func (in *Inspector) FindNodes(typ reflect.Type) []Node {
	var nodes []Node
	w := NewWalker(in.log, in.ast)
	w.AddVisitor(typ, func(log lg.Log, w *Walker, node Node) error {
		nodes = append(nodes, node)
		return nil
	})

	if err := w.Walk(); err != nil {
		// Should never happen
		panic(err)
	}
	return nodes
}

// FindHandles returns all handles mentioned in the AST.
func (in *Inspector) FindHandles() []string {
	var handles []string

	if err := walkWith(in.log, in.ast, typeHandleNode, func(log lg.Log, walker *Walker, node Node) error {
		handles = append(handles, node.Text())
		return nil
	}); err != nil {
		panic(err)
	}

	if err := walkWith(in.log, in.ast, typeTblSelectorNode, func(log lg.Log, walker *Walker, node Node) error {
		n, _ := node.(*TblSelectorNode)
		if n.handle != "" {
			handles = append(handles, n.handle)
		}
		return nil
	}); err != nil {
		panic(err)
	}

	return lo.Uniq(handles)
}

// FindWhereClauses returns all the WHERE clauses in the AST.
func (in *Inspector) FindWhereClauses() ([]*WhereNode, error) {
	var nodes []*WhereNode

	for _, seg := range in.ast.Segments() {
		// WhereNode clauses must be the only child of a segment
		if len(seg.Children()) == 1 {
			if w, ok := seg.Children()[0].(*WhereNode); ok {
				nodes = append(nodes, w)
			}
		}
	}

	return nodes, nil
}

// FindColExprSegment returns the segment containing col expressions (such as
// ".uid, .email"). This is typically the last segment. It's also possible that
// there is no such segment (which usually results in a SELECT * FROM).
func (in *Inspector) FindColExprSegment() (*SegmentNode, error) {
	segs := in.ast.Segments()

	// work backwards from the end
	for i := len(segs) - 1; i > 0; i-- {
		elems := segs[i].Children()
		numColExprs := 0

		for _, elem := range elems {
			if _, ok := elem.(ResultColumn); !ok {
				if numColExprs > 0 {
					return nil, errorf("found non-homogenous col expr segment [%d]: also has element %T", i, elem)
				}

				// else it's not a col expr segment, break
				break
			}

			numColExprs++
		}

		if numColExprs > 0 {
			return segs[i], nil
		}
	}

	return nil, nil //nolint:nilnil
}

// FindOrderByNode returns the OrderByNode, or nil if not found.
func (in *Inspector) FindOrderByNode() (*OrderByNode, error) {
	segs := in.ast.Segments()

	for i := range segs {
		nodes := nodesWithType(segs[i].Children(), typeOrderByNode)
		switch len(nodes) {
		case 0:
			// No OrderByNode in this segment, continue searching.
			continue
		case 1:
			// Found it
			node, _ := nodes[0].(*OrderByNode)
			return node, nil
		default:
			// Shouldn't be possible
			return nil, errorf("Segment {%s} has %d OrderByNode children, but should have a max of 1", segs[i])
		}
	}

	return nil, nil //nolint:nilnil
}

// FindGroupByNode returns the GroupByNode, or nil if not found.
func (in *Inspector) FindGroupByNode() (*GroupByNode, error) {
	segs := in.ast.Segments()

	for i := range segs {
		nodes := nodesWithType(segs[i].Children(), typeGroupByNode)
		switch len(nodes) {
		case 0:
			// No GroupByNode in this segment, continue searching.
			continue
		case 1:
			// Found it
			node, _ := nodes[0].(*GroupByNode)
			return node, nil
		default:
			// Shouldn't be possible
			return nil, errorf("Segment {%s} has %d GroupByNode children, but should have a max of 1", segs[i])
		}
	}

	return nil, nil //nolint:nilnil
}

// FindTablerSegments returns the segments that have at least one child
// that implements Tabler.
func (in *Inspector) FindTablerSegments() []*SegmentNode {
	segs := in.ast.Segments()
	selSegs := make([]*SegmentNode, 0, 2)

	for _, seg := range segs {
		for _, child := range seg.Children() {
			if _, ok := child.(Tabler); ok {
				selSegs = append(selSegs, seg)
				break
			}
		}
	}

	return selSegs
}

// FindFinalTablerSegment returns the final segment that
// has at lest one child that implements Tabler.
func (in *Inspector) FindFinalTablerSegment() (*SegmentNode, error) {
	selectableSegs := in.FindTablerSegments()
	if len(selectableSegs) == 0 {
		return nil, errorf("no selectable segments")
	}
	selectableSeg := selectableSegs[len(selectableSegs)-1]
	return selectableSeg, nil
}

// FindUniqueNode returns any UniqueNode, or nil.
func (in *Inspector) FindUniqueNode() (*UniqueNode, error) {
	nodes := in.FindNodes(typeUniqueNode)
	if len(nodes) == 0 {
		return nil, nil //nolint:nilnil
	}
	return nodes[0].(*UniqueNode), nil
}
