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
		if err := visitFn(w, node); err != nil {
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
