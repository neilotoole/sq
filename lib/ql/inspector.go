package ql

import "reflect"

type Inspector struct {
	ast *AST
}

func NewInspector(ast *AST) *Inspector {
	return &Inspector{ast: ast}
}

func (in *Inspector) findDatasource() (string, error) {

	var ds string
	w := NewWalker(in.ast)
	w.AddVisitor(TypeDatasource, func(w *Walker, node Node) error {
		ds = node.(*Datasource).Text()
		return nil
	})

	err := w.Walk()
	return ds, err

}

func (in *Inspector) countNodes(typ NodeType) int {

	count := 0
	w := NewWalker(in.ast)
	w.AddVisitor(typ, func(w *Walker, node Node) error {
		count++
		return nil
	})

	w.Walk()
	return count

}

func (in *Inspector) findNodes(typ NodeType) []Node {

	nodes := []Node{}
	w := NewWalker(in.ast)
	w.AddVisitor(typ, func(w *Walker, node Node) error {
		nodes = append(nodes, node)
		return nil
	})

	w.Walk()
	return nodes

}

// findColExprSegment returns the segment containing col expressions (such as
// ".uid, .email"). This is typically the last segment. It's also possible that
// there is no such segment (which usualy results in a SELECT * FROM).
func (in *Inspector) findColExprSegment() (*Segment, error) {

	segs := in.ast.Segments()

	//if len(segs) < 2 {
	//	// there's always the datasource and tbl expr segment, so if less
	//	// than 3 segments, there can't be a col expr segment
	//	return nil, nil
	//}

	// work backwards from the end
	for i := len(segs) - 1; i > 0; i-- {

		elems := segs[i].Children()
		numColExprs := 0

		for _, elem := range elems {

			isColExpr := reflect.TypeOf(elem).Implements(reflect.Type(TypeColExpr))
			if isColExpr == false {
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

	return nil, nil
}

// findSelectableSegments returns the segments that have at least one child
// that implements Selectable.
func (in *Inspector) findSelectableSegments() []*Segment {

	segs := in.ast.Segments()
	selSegs := make([]*Segment, 0, 2)

	for _, seg := range segs {
		for _, child := range seg.Children() {

			childType := reflect.TypeOf(child)
			if childType.Implements(reflect.Type(TypeSelectable)) {
				selSegs = append(selSegs, seg)
				break
			}
		}
	}

	return selSegs
}

func (in *Inspector) findFinalSelectableSegment() (*Segment, error) {
	selectableSegs := in.findSelectableSegments()
	if len(selectableSegs) == 0 {
		return nil, errorf("no selectable segments")
	}
	selectableSeg := selectableSegs[len(selectableSegs)-1]
	return selectableSeg, nil
}
