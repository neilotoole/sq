package ast

import "github.com/neilotoole/go-lg/lg"

// narrowTblSel takes a generic selector, and if appropriate, converts it to a TblSel.
func narrowTblSel(w *Walker, node Node) error {

	// node is guaranteed to be TypeSelector
	sel, ok := node.(*Selector)
	if !ok {
		return errorf("expected *Selector but got %T", node)
	}

	lg.Debugf("visiting selector: %q", sel.Text())

	seg, ok := sel.Parent().(*Segment)
	if !ok {
		lg.Debugf("parent is not a segment, but is %T", sel.Parent())
		return nil
	}

	if seg.SegIndex() == 0 {
		return errorf("syntax error: illegal to have raw selector in first segment: %q", sel.Text())
	}

	typ, err := seg.Prev().ChildType()
	if err != nil {
		return err
	}

	if typ == TypeDatasource {
		ds, ok := seg.Prev().Children()[0].(*Datasource)
		if !ok {
			return errorf("syntax error: expected Datasource, but got %T", seg.Prev().Children()[0])
		}

		// this means that this selector must be a table selector
		tblSel := newTblSelector(seg, sel.SelValue(), sel.Context())
		tblSel.DSName = ds.Text()
		err := ReplaceNode(sel, tblSel)
		if err != nil {
			return err
		}
	}

	return nil
}

// narrowColSel takes a generic selector, and if appropriate, converts it to a ColSel.
func narrowColSel(w *Walker, node Node) error {

	// node is guaranteed to be TypeSelector
	sel, ok := node.(*Selector)
	if !ok {
		return errorf("expected *Selector but got %T", node)
	}

	lg.Debugf("visiting selector: %q", sel.Text())

	parent := sel.Parent()

	switch parent := parent.(type) {
	case *JoinConstraint:
		lg.Debugf("selector parent is *FnJoinExpr, therefore this is a ColSel")
		colSel := newColSelector(sel.Parent(), sel.ctx)
		return ReplaceNode(sel, colSel)
	case *Segment:
		lg.Debugf("parent is *Segment")
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final selectable seg are
		// convert to colSels.
		selectableSeg, err := NewInspector(w.root.(*AST)).FindFinalSelectableSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= selectableSeg.SegIndex() {
			lg.Debugf("skipping this selector because it's not after the final selectable segment")
			return nil
		}

		colSel := newColSelector(sel.Parent(), sel.ctx)
		return ReplaceNode(sel, colSel)

	default:
		lg.Warnf("skipping this selector, as parent is not of a relevant type, but is %T", parent)
	}

	return nil
}

// determineJoinTables attempts to determine the tables that a JOIN refers to
func determineJoinTables(w *Walker, node Node) error {

	// node is guaranteed to be FnJoin
	fnJoin, ok := node.(*Join)
	if !ok {
		return errorf("expected *FnJoin but got %T", node)
	}

	seg, ok := fnJoin.Parent().(*Segment)
	if !ok {
		return errorf("JOIN() must have a *Segment parent, but got %T", fnJoin.Parent())
	}

	prevSeg := seg.Prev()
	if prevSeg == nil {
		return errorf("JOIN() must not be in the first segment")
	}

	childType, err := prevSeg.ChildType()
	if err != nil {
		return err
	}

	if childType != TypeTableSelector || len(prevSeg.Children()) != 2 {
		return errorf("JOIN() must have two table selectors in the preceding segment")
	}

	fnJoin.leftTbl, ok = prevSeg.Children()[0].(*TblSelector)
	if !ok {
		return errorf("JOIN() expected table selector in previous segment, but was %T(%q)", prevSeg.Children()[0], prevSeg.Children()[0].Text())
	}
	fnJoin.rightTbl, ok = prevSeg.Children()[1].(*TblSelector)
	if !ok {
		return errorf("JOIN() expected table selector in previous segment, but was %T(%q)", prevSeg.Children()[1], prevSeg.Children()[1].Text())
	}
	return nil
}

// visitCheckRowRange validates the RowRange element
func visitCheckRowRange(w *Walker, node Node) error {

	// node is guaranteed to be FnJoin
	rr, ok := node.(*RowRange)
	if !ok {
		return errorf("expected %s but got %T", TypeRowRange, node)
	}

	if w.state != nil {
		return errorf("only one row range element permitted")
	}

	w.state = rr
	// TODO: check that the row range is after a selectable
	return nil
}
