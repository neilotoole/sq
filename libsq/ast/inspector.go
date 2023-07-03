package ast

import (
	"reflect"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/samber/lo"
)

// Inspector provides functionality for AST interrogation.
type Inspector struct {
	ast *AST
}

// NewInspector returns an Inspector instance for ast.
func NewInspector(ast *AST) *Inspector {
	return &Inspector{ast: ast}
}

// CountNodes counts the number of nodes having typ.
func (in *Inspector) CountNodes(typ reflect.Type) int {
	count := 0
	w := NewWalker(in.ast)
	w.AddVisitor(typ, func(w *Walker, node Node) error {
		count++
		return nil
	})

	_ = w.Walk()
	return count
}

// FindNodes returns the nodes having typ.
func (in *Inspector) FindNodes(typ reflect.Type) []Node {
	var nodes []Node
	w := NewWalker(in.ast)
	w.AddVisitor(typ, func(w *Walker, node Node) error {
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

	if err := walkWith(in.ast, typeHandleNode, func(walker *Walker, node Node) error {
		handles = append(handles, node.Text())
		return nil
	}); err != nil {
		panic(err)
	}

	if err := walkWith(in.ast, typeTblSelectorNode, func(walker *Walker, node Node) error {
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
	for i := len(segs) - 1; i >= 0; i-- {
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
			return nil, errorf("segment {%s} has %d OrderByNode children, but max is 1",
				segs[i], len(nodes))
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
			return nil, errorf("segment {%s} has %d GroupByNode children, but max is 1",
				segs[i], len(nodes))
		}
	}

	return nil, nil //nolint:nilnil
}

// FindTableSegments returns the segments that have at least one child
// that is a ast.TblSelectorNode.
func (in *Inspector) FindTableSegments() []*SegmentNode {
	segs := in.ast.Segments()
	selSegs := make([]*SegmentNode, 0, 2)

	for _, seg := range segs {
		for _, child := range seg.Children() {
			if _, ok := child.(*TblSelectorNode); ok {
				selSegs = append(selSegs, seg)
				break
			}
		}
	}

	return selSegs
}

// FindFirstHandle returns the first handle mentioned in the query,
// or returns empty string.
func (in *Inspector) FindFirstHandle() (handle string) {
	nodes := in.FindNodes(typeHandleNode)
	if len(nodes) > 0 {
		handle = nodes[0].(*HandleNode).Handle()
		return handle
	}

	nodes = in.FindNodes(typeTblSelectorNode)
	for _, node := range nodes {
		handle = node.(*TblSelectorNode).Handle()
		if handle != "" {
			return handle
		}
	}

	return ""
}

// FindFirstTableSelector returns the first top-level (child of a segment)
// table selector node.
func (in *Inspector) FindFirstTableSelector() *TblSelectorNode {
	segs := in.ast.Segments()
	if len(segs) == 0 {
		return nil
	}

	var tblSelNode *TblSelectorNode
	var ok bool

	for _, seg := range segs {
		for _, child := range seg.Children() {
			if tblSelNode, ok = child.(*TblSelectorNode); ok {
				return tblSelNode
			}
		}
	}

	return nil
}

// FindFinalTableSegment returns the final segment that
// has at least one child that is an ast.TblSelectorNode.
func (in *Inspector) FindFinalTableSegment() (*SegmentNode, error) {
	selectableSegs := in.FindTableSegments()
	if len(selectableSegs) == 0 {
		return nil, errorf("no selectable segments")
	}
	selectableSeg := selectableSegs[len(selectableSegs)-1]
	return selectableSeg, nil
}

// FindJoins returns all ast.JoinNode instances.
func (in *Inspector) FindJoins() ([]*JoinNode, error) {
	nodes := in.FindNodes(typeJoinNode)
	joinNodes := make([]*JoinNode, len(nodes))
	var ok bool
	for i := range nodes {
		joinNodes[i], ok = nodes[i].(*JoinNode)
		if !ok {
			return nil, errz.Errorf("expected %T but got %T", (*JoinNode)(nil), nodes[i])
		}
	}

	return joinNodes, nil
}

// FindUniqueNode returns any UniqueNode, or nil.
func (in *Inspector) FindUniqueNode() (*UniqueNode, error) {
	nodes := in.FindNodes(typeUniqueNode)
	if len(nodes) == 0 {
		return nil, nil //nolint:nilnil
	}
	return nodes[0].(*UniqueNode), nil
}

// FindRowRangeNode returns the single RowRangeNode, or nil.
// An error can be returned if the AST is in an illegal state.
func (in *Inspector) FindRowRangeNode() (*RowRangeNode, error) {
	nodes := in.FindNodes(typeRowRangeNode)
	switch len(nodes) {
	case 0:
		return nil, nil //nolint:nilnil
	case 1:
		return nodes[0].(*RowRangeNode), nil
	default:
		// Shouldn't be possible
		return nil, errorf("illegal query: only one %T allowed, but found %d", (*RowRangeNode)(nil), len(nodes))
	}
}
