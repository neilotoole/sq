package ast

import (
	"reflect"
)

// nodeVisitorFn is a visitor function that the walker invokes for each node it visits.
type nodeVisitorFn func(*Walker, Node) error

// Walker traverses a node tree (the AST, or a subset thereof).
type Walker struct {
	root     Node
	visitors map[reflect.Type][]nodeVisitorFn
	// state is a generic field to hold any data that a visitor function
	// might need to stash on the walker.
	state any
}

// NewWalker returns a new Walker instance.
func NewWalker(node Node) *Walker {
	w := &Walker{root: node}
	w.visitors = map[reflect.Type][]nodeVisitorFn{}
	return w
}

// AddVisitor adds a visitor function for any node that is assignable
// to typ.
func (w *Walker) AddVisitor(typ reflect.Type, visitor nodeVisitorFn) *Walker {
	funcs := w.visitors[typ]
	if funcs == nil {
		funcs = []nodeVisitorFn{}
	}

	funcs = append(funcs, visitor)
	w.visitors[typ] = funcs
	return w
}

// Walk starts the walking process.
func (w *Walker) Walk() error {
	return w.visit(w.root)
}

func (w *Walker) visit(node Node) error {
	var visitFns []nodeVisitorFn
	nodeType := reflect.TypeOf(node)
	for fnType, fns := range w.visitors {
		if nodeType.AssignableTo(fnType) {
			visitFns = append(visitFns, fns...)
		}
	}

	for _, visitFn := range visitFns {
		err := visitFn(w, node)
		if err != nil {
			return err
		}
	}

	return w.visitChildren(node)
}

func (w *Walker) visitChildren(node Node) error {
	for _, child := range node.Children() {
		err := w.visit(child)
		if err != nil {
			return err
		}
	}

	return nil
}

// walkWith is a convenience function for using Walker.
func walkWith(ast *AST, typ reflect.Type, fn nodeVisitorFn) error {
	return NewWalker(ast).AddVisitor(typ, fn).Walk()
}

// narrowTblSel takes a generic selector, and if appropriate, converts it to a TblSel.
func narrowTblSel(_ *Walker, node Node) error {
	// node is guaranteed to be typeSelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected *SelectorNode but got %T", node)
	}

	seg, ok := sel.Parent().(*SegmentNode)
	if !ok {
		return nil
	}

	if seg.SegIndex() == 0 {
		return errorf("@HANDLE must be first element: %s", sel.Text())
	}

	prevType, err := seg.Prev().ChildType()
	if err != nil {
		return err
	}

	if prevType == typeHandleNode {
		handleNode, ok := seg.Prev().Children()[0].(*HandleNode)
		if !ok {
			return errorf("syntax error: expected HandleNode, but got %T", seg.Prev().Children()[0])
		}

		// this means that this selector must be a table selector
		tblSel, err := newTblSelector(sel)
		if err != nil {
			return err
		}
		tblSel.handle = handleNode.Text()
		err = nodeReplace(sel, tblSel)
		if err != nil {
			return err
		}
	}

	return nil
}

// narrowTblColSel takes a generic selector, and if appropriate, replaces it
// with a TblColSelectorNode.
func narrowTblColSel(w *Walker, node Node) error {
	// node is guaranteed to be type SelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected %T but got %T", sel, node)
	}

	parent := sel.Parent()
	switch parent := parent.(type) {
	case *JoinConstraint, *FuncNode, *OrderByTermNode, *GroupByNode, *ExprNode:
		if sel.name1 == "" {
			return nil
		}

		tblColSelNode, err := newTblColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, tblColSelNode)
	case *SegmentNode:
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final selectable seg are
		// convert to TblColSelectorNode.
		selectableSeg, err := NewInspector(w.root.(*AST)).FindFinalTablerSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= selectableSeg.SegIndex() {
			// Skipping this selector because it's not after the final selectable segment
			return nil
		}

		if sel.name1 == "" {
			// It's only a TblColSelectorNode if name1 is set.
			// So, it's either a ColSelectorNode or a TblSelectorNode.
			return nil
		}

		tblColSelNode, err := newTblColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, tblColSelNode)

	default:
	}

	return nil
}

// narrowColSel takes a generic selector, and if appropriate, converts it to a ColSel.
func narrowColSel(w *Walker, node Node) error {
	// node is guaranteed to be type SelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected %T but got %T", sel, node)
	}

	parent := sel.Parent()

	switch parent := parent.(type) {
	case *JoinConstraint, *FuncNode, *OrderByTermNode, *GroupByNode, *ExprNode:
		colSel, err := newColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, colSel)
	case *SegmentNode:
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final selectable seg are
		// convert to colSels.
		selectableSeg, err := NewInspector(w.root.(*AST)).FindFinalTablerSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= selectableSeg.SegIndex() {
			// Skipping this selector because it's not after the final selectable segment
			return nil
		}

		colSel, err := newColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, colSel)

	default:
		// Skipping this selector, as parent is not of a relevant type
	}

	return nil
}

// determineJoinTables attempts to determine the tables that a JOIN refers to.
func determineJoinTables(_ *Walker, node Node) error {
	// node is guaranteed to be FnJoin
	fnJoin, ok := node.(*JoinNode)
	if !ok {
		return errorf("expected *FnJoin but got %T", node)
	}

	seg, ok := fnJoin.Parent().(*SegmentNode)
	if !ok {
		return errorf("JOIN() must have a *SegmentNode parent, but got %T", fnJoin.Parent())
	}

	prevSeg := seg.Prev()
	if prevSeg == nil {
		return errorf("JOIN() must not be in the first segment")
	}

	if len(prevSeg.Children()) != 2 || len(nodesWithType(prevSeg.Children(), typeTblSelectorNode)) != 2 {
		return errorf("JOIN() must have two table selectors in the preceding segment")
	}

	fnJoin.leftTbl, ok = prevSeg.Children()[0].(*TblSelectorNode)
	if !ok {
		return errorf("JOIN() expected table selector in previous segment, but was %T(%s)", prevSeg.Children()[0],
			prevSeg.Children()[0].Text())
	}
	fnJoin.rightTbl, ok = prevSeg.Children()[1].(*TblSelectorNode)
	if !ok {
		return errorf("JOIN() expected table selector in previous segment, but was %T(%s)", prevSeg.Children()[1],
			prevSeg.Children()[1].Text())
	}
	return nil
}

// visitCheckRowRange validates the RowRangeNode element.
func visitCheckRowRange(w *Walker, node Node) error {
	// node is guaranteed to be FnJoin
	rr, ok := node.(*RowRangeNode)
	if !ok {
		return errorf("expected %s but got %T", typeRowRangeNode, node)
	}

	if w.state != nil {
		return errorf("only one row range element permitted")
	}

	w.state = rr
	// TODO: check that the row range is after a selectable
	return nil
}
