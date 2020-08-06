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
	state interface{}
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
	// node is guaranteed to be typeSelector
	sel, ok := node.(*Selector)
	if !ok {
		return errorf("expected *Selector but got %T", node)
	}

	seg, ok := sel.Parent().(*Segment)
	if !ok {
		log.Debugf("parent is not a segment, but is %T", sel.Parent())
		return nil
	}

	if seg.SegIndex() == 0 {
		return errorf("syntax error: illegal to have raw selector in first segment: %q", sel.Text())
	}

	typ, err := seg.Prev().ChildType()
	if err != nil {
		return err
	}

	if typ == typeDatasource {
		ds, ok := seg.Prev().Children()[0].(*Datasource)
		if !ok {
			return errorf("syntax error: expected Datasource, but got %T", seg.Prev().Children()[0])
		}

		// this means that this selector must be a table selector
		tblSel := newTblSelector(seg, sel.SelValue(), sel.Context())
		tblSel.DSName = ds.Text()
		err = replaceNode(sel, tblSel)
		if err != nil {
			return err
		}
	}

	return nil
}

// narrowColSel takes a generic selector, and if appropriate, converts it to a ColSel.
func narrowColSel(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be typeSelector
	sel, ok := node.(*Selector)
	if !ok {
		return errorf("expected *Selector but got %T", node)
	}

	parent := sel.Parent()

	switch parent := parent.(type) {
	case *JoinConstraint, *Func:
		// selector parent is JoinConstraint or Func, therefore this is a ColSelector
		log.Debugf("selector parent is %T, therefore this is a ColSelector", parent)
		colSel := newColSelector(sel.Parent(), sel.ctx)
		return replaceNode(sel, colSel)
	case *Segment:
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

		colSel := newColSelector(sel.Parent(), sel.ctx)
		return replaceNode(sel, colSel)

	default:
		log.Warnf("skipping this selector, as parent is not of a relevant type, but is %T", parent)
	}

	return nil
}

// findWhereClause locates any expressions that represent the WHERE clause of the SQL SELECT stmt, and
// inserts a SetWhere node into the AST for that expression.
//
// In practice, a WHERE clause is an *Expr that is the only child of a segment. For example:
//    @my1 | .tbluser | .uid > 4 | .uid, .email
// In this case, ".uid > 4" is the WHERE clause.
func findWhereClause(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be *Expr
	expr, ok := node.(*Expr)
	if !ok {
		return errorf("expected *Expr but got %T", node)
	}

	log.Debugf("found expression: %q", expr.Context().GetText())

	seg, ok := expr.Parent().(*Segment)
	if !ok {
		// The expr is not the direct child of a segment, so we're not interested in it.
		return nil
	}

	log.Debugf("expr parent is *Segment")
	if len(seg.Children()) != 1 {
		return errorf("Segment with expression - representing a WHERE clause - must only have one child")
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

// determineJoinTables attempts to determine the tables that a JOIN refers to
func determineJoinTables(log lg.Log, w *Walker, node Node) error {
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

	if len(prevSeg.Children()) != 2 || len(nodesWithType(prevSeg.Children(), typeTblSelector)) != 2 {
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
func visitCheckRowRange(log lg.Log, w *Walker, node Node) error {
	// node is guaranteed to be FnJoin
	rr, ok := node.(*RowRange)
	if !ok {
		return errorf("expected %s but got %T", typeRowRange, node)
	}

	if w.state != nil {
		return errorf("only one row range element permitted")
	}

	w.state = rr
	// TODO: check that the row range is after a selectable
	return nil
}
