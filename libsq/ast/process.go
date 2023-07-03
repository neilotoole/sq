package ast

// narrowTblSel takes a generic selector, and if appropriate, converts it
// to an *ast.TblSelectorNode.
func narrowTblSel(_ *Walker, node Node) error {
	// node is guaranteed to be a selector node
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected %T but got %T", sel, node)
	}

	seg, ok := sel.Parent().(*SegmentNode)
	if !ok {
		return nil
	}

	var (
		tblSel *TblSelectorNode
		err    error
	)

	if seg.SegIndex() == 0 {
		// If this is the first segment, then the selector MUST refer
		// to a table. E.g.
		//
		//  $ sq '.data'
		//
		// ".data" must be a table.
		if tblSel, err = newTblSelector(sel); err != nil {
			return err
		}

		return nodeReplace(sel, tblSel)
	}

	prevType, err := seg.Prev().ChildType()
	if err != nil {
		return err
	}

	if prevType == typeHandleNode {
		var handleNode *HandleNode
		if handleNode, ok = seg.Prev().Children()[0].(*HandleNode); !ok {
			return errorf("syntax error: expected %T, but got %T", handleNode, seg.Prev().Children()[0])
		}

		// this means that this selector must be a table selector
		if tblSel, err = newTblSelector(sel); err != nil {
			return err
		}
		tblSel.handle = handleNode.Text()
		return nodeReplace(sel, tblSel)
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
	case *FuncNode, *OrderByTermNode, *GroupByNode, *ExprNode:
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
		// Only top-level selectors after the final table seg are
		// converted to TblColSelectorNode.
		tblSeg, err := NewInspector(w.root.(*AST)).FindFinalTableSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= tblSeg.SegIndex() {
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
	case *FuncNode, *OrderByTermNode, *GroupByNode, *ExprNode:
		colSel, err := newColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, colSel)
	case *SegmentNode:
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final table seg are
		// convert to colSels.
		tblSeg, err := NewInspector(w.root.(*AST)).FindFinalTableSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= tblSeg.SegIndex() {
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
