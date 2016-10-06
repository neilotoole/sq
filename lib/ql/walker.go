package ql

import "reflect"

type NodeVisitorFn func(*Walker, Node) error

type Walker struct {
	root     Node
	visitors map[NodeType][]NodeVisitorFn
	// state is a generic field to hold any data that a visitor function
	// might need to stash on the walker.
	state interface{}
}

func NewWalker(node Node) *Walker {

	w := &Walker{root: node}
	w.visitors = make(map[NodeType][]NodeVisitorFn)
	return w
}

// AddVisitor adds a visitor function for the specified node type (and returns
// the receiver Walker, to enabled chaining).
func (w *Walker) AddVisitor(typ NodeType, visitor NodeVisitorFn) *Walker {

	funcs := w.visitors[typ]
	if funcs == nil {
		funcs = []NodeVisitorFn{}
	}

	funcs = append(funcs, visitor)

	w.visitors[typ] = funcs
	return w
}

func (w *Walker) Walk() error {

	return w.Visit(w.root)
}

func (w *Walker) Visit(node Node) error {

	//typ := NodeType(reflect.TypeOf(node).Elem())
	typ := NodeType(reflect.TypeOf(node))
	//lg.Debugf("%s -- %s", typ, node)
	visitFns, ok := w.visitors[typ]

	//lg.Debugf("visiting node of type %T with %d visitor function(s)", node, len(visitFns))

	if ok {
		for _, visitFn := range visitFns {
			err := visitFn(w, node)
			if err != nil {
				return err
			}
		}

		return nil
	}

	//if visitFn != nil {
	//	return visitFn(w, node)
	//}

	return w.VisitChildren(node)
}

func (w *Walker) VisitChildren(node Node) error {

	for _, child := range node.Children() {
		err := w.Visit(child)
		if err != nil {
			return err
		}
	}

	return nil
}
