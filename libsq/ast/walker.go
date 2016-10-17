package ast

import "reflect"

// NodeVisitorFn is a visitor function that the walker invokes for each node it visits.
type NodeVisitorFn func(*Walker, Node) error

// Walker traverses an AST tree (or a subset thereof).
type Walker struct {
	root     Node
	visitors map[NodeType][]NodeVisitorFn
	// state is a generic field to hold any data that a visitor function
	// might need to stash on the walker.
	state interface{}
}

// NewWalker returns a new Walker instance.
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

// Walk starts the walking process.
func (w *Walker) Walk() error {
	return w.visit(w.root)
}

func (w *Walker) visit(node Node) error {
	typ := NodeType(reflect.TypeOf(node))
	visitFns, ok := w.visitors[typ]

	if ok {
		for _, visitFn := range visitFns {
			err := visitFn(w, node)
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
