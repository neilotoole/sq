package ast

import (
	"reflect"

	"github.com/neilotoole/lg"
)

// nodeVisitorFn is a visitor function that the walker invokes for each node it visits.
type nodeVisitorFn func(lg.Log, *Walker, Node) error

// Walker traverses a node tree (the AST, or a subset thereof).
type Walker struct {
	log      lg.Log
	root     Node
	visitors map[reflect.Type][]nodeVisitorFn
	// state is a generic field to hold any data that a visitor function
	// might need to stash on the walker.
	state any
}

// NewWalker returns a new Walker instance.
func NewWalker(log lg.Log, node Node) *Walker {
	w := &Walker{log: log, root: node}
	w.visitors = map[reflect.Type][]nodeVisitorFn{}
	return w
}

// AddVisitor adds a visitor function for the specified node type (and returns
// the receiver Walker, to enabled chaining).
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
	typ := reflect.TypeOf(node)
	visitFns, ok := w.visitors[typ]

	if ok {
		for _, visitFn := range visitFns {
			err := visitFn(w.log, w, node)
			if err != nil {
				return err
			}
		}
		return nil
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

// narrowTblSel takes a generic selector, and if appropriate, converts it to a TblSel.
func narrowTblSel(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be typeSelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected *SelectorNode but got %T", node)
	}

	seg, ok := sel.Parent().(*SegmentNode)
	if !ok {
		log.Debugf("parent is not a segment, but is %T", sel.Parent())
		return nil
	}

	if seg.SegIndex() == 0 {
		return errorf("@HANDLE must be first element: %q", sel.Text())
	}

	prevType, err := seg.Prev().ChildType()
	if err != nil {
		return err
	}
	log.Debugf("prevType: %s", prevType)

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
		tblSel.Handle = handleNode.Text()
		err = nodeReplace(sel, tblSel)
		if err != nil {
			return err
		}
	}

	return nil
}

// narrowTblColSel takes a generic selector, and if appropriate, replaces it
// with a TblColSelectorNode.
func narrowTblColSel(log lg.Log, w *Walker, node Node) error { //nolint:dupl
	// node is guaranteed to be type SelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected *SelectorNode but got %T", node)
	}

	parent := sel.Parent()
	switch parent := parent.(type) {
	case *JoinConstraint, *Func:
		// selector parent is JoinConstraint or Func, therefore this is a ColSelectorNode
		log.Debugf("selector parent is %T, therefore this is a TblColSelectorNode", parent)
		tblColSelNode, err := newTblColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, tblColSelNode)
	case *SegmentNode:
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final selectable seg are
		// convert to colSels.
		selectableSeg, err := NewInspector(log, w.root.(*AST)).FindFinalSelectableSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= selectableSeg.SegIndex() {
			log.Debugf("skipping this selector because it's not after the final selectable segment")
			return nil
		}

		tblColSelNode, err := newTblColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, tblColSelNode)

	default:
		log.Warnf("skipping this selector, as parent is not of a relevant type, but is %T", parent)
	}

	return nil
}

// narrowColSel takes a generic selector, and if appropriate, converts it to a ColSel.
func narrowColSel(log lg.Log, w *Walker, node Node) error { //nolint:dupl
	// node is guaranteed to be type SelectorNode
	sel, ok := node.(*SelectorNode)
	if !ok {
		return errorf("expected *SelectorNode but got %T", node)
	}

	parent := sel.Parent()

	switch parent := parent.(type) {
	case *JoinConstraint, *Func:
		// selector parent is JoinConstraint or Func, therefore this is a ColSelectorNode
		log.Debugf("selector parent is %T, therefore this is a ColSelectorNode", parent)
		colSel, err := newColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, colSel)
	case *SegmentNode:
		// if the parent is a segment, this is a "top-level" selector.
		// Only top-level selectors after the final selectable seg are
		// convert to colSels.
		selectableSeg, err := NewInspector(log, w.root.(*AST)).FindFinalSelectableSegment()
		if err != nil {
			return err
		}

		if parent.SegIndex() <= selectableSeg.SegIndex() {
			log.Debugf("skipping this selector because it's not after the final selectable segment")
			return nil
		}

		colSel, err := newColSelectorNode(sel)
		if err != nil {
			return err
		}
		return nodeReplace(sel, colSel)

	default:
		log.Warnf("skipping this selector, as parent is not of a relevant type, but is %T", parent)
	}

	return nil
}

// findWhereClause locates any expressions that represent the WHERE clause of the SQL SELECT stmt, and
// inserts a SetWhere node into the AST for that expression.
//
// In practice, a WHERE clause is an *Expr that is the only child of a segment. For example:
//
//	@my1 | .tbluser | .uid > 4 | .uid, .email
//
// In this case, ".uid > 4" is the WHERE clause.
func findWhereClause(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be *Expr
	expr, ok := node.(*Expr)
	if !ok {
		return errorf("expected *Expr but got %T", node)
	}

	log.Debugf("found expression: %q", expr.Context().GetText())

	seg, ok := expr.Parent().(*SegmentNode)
	if !ok {
		// The expr is not the direct child of a segment, so we're not interested in it.
		return nil
	}

	log.Debugf("expr parent is *SegmentNode")
	if len(seg.Children()) != 1 {
		return errorf("SegmentNode with expression - representing a WHERE clause - must only have one child")
	}

	// The expr is the direct and only child of a segment.
	// We insert a Where between the segment and the expr.
	where := &Where{}
	where.ctx = expr.ctx
	err := where.AddChild(expr)
	if err != nil {
		return err
	}
	err = expr.SetParent(where)
	if err != nil {
		return err
	}

	seg.bn.children[0] = where
	return nil
}

// determineJoinTables attempts to determine the tables that a JOIN refers to.
func determineJoinTables(log lg.Log, w *Walker, node Node) error {
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
		return errorf("JOIN() expected table selector in previous segment, but was %T(%q)", prevSeg.Children()[0],
			prevSeg.Children()[0].Text())
	}
	fnJoin.rightTbl, ok = prevSeg.Children()[1].(*TblSelectorNode)
	if !ok {
		return errorf("JOIN() expected table selector in previous segment, but was %T(%q)", prevSeg.Children()[1],
			prevSeg.Children()[1].Text())
	}
	return nil
}

// visitCheckRowRange validates the RowRange element.
func visitCheckRowRange(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be FnJoin
	rr, ok := node.(*RowRange)
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
