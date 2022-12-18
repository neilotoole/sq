package ast

import (
	"reflect"

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
		return nil
	})

	_ = w.Walk()
	return count
}

// FindNodes returns all of the nodes having typ.
func (in *Inspector) FindNodes(typ reflect.Type) []Node {
	var nodes []Node
	w := NewWalker(in.log, in.ast)
	w.AddVisitor(typ, func(log lg.Log, w *Walker, node Node) error {
		nodes = append(nodes, node)
		return nil
	})

	_ = w.Walk()
	return nodes
}

// FindWhereClauses returns all the WHERE clauses in the AST.
func (in *Inspector) FindWhereClauses() ([]*Where, error) {
	ws := []*Where{}

	for _, seg := range in.ast.Segments() {
		// Where clauses must be the only child of a segment
		if len(seg.Children()) == 1 {
			if w, ok := seg.Children()[0].(*Where); ok {
				ws = append(ws, w)
			}
		}
	}

	return ws, nil
}

// FindColExprSegment returns the segment containing col expressions (such as
// ".uid, .email"). This is typically the last segment. It's also possible that
// there is no such segment (which usually results in a SELECT * FROM).
func (in *Inspector) FindColExprSegment() (*Segment, error) {
	segs := in.ast.Segments()

	// work backwards from the end
	for i := len(segs) - 1; i > 0; i-- {
		elems := segs[i].Children()
		numColExprs := 0

		for _, elem := range elems {
			if _, ok := elem.(ColExpr); !ok {
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

// FindSelectableSegments returns the segments that have at least one child
// that implements Selectable.
func (in *Inspector) FindSelectableSegments() []*Segment {
	segs := in.ast.Segments()
	selSegs := make([]*Segment, 0, 2)

	for _, seg := range segs {
		for _, child := range seg.Children() {
			if _, ok := child.(Selectable); ok {
				selSegs = append(selSegs, seg)
				break
			}
		}
	}

	return selSegs
}

// FindFinalSelectableSegment returns the final segment that
// has at lest one child that implements Selectable.
func (in *Inspector) FindFinalSelectableSegment() (*Segment, error) {
	selectableSegs := in.FindSelectableSegments()
	if len(selectableSegs) == 0 {
		return nil, errorf("no selectable segments")
	}
	selectableSeg := selectableSegs[len(selectableSegs)-1]
	return selectableSeg, nil
}
