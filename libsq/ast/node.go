package ast

import (
	"fmt"
	"reflect"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

// Node is an AST node.
type Node interface {
	// Parent returns the node's parent, which may be nil..
	Parent() Node

	// SetParent sets the node's parent, returning an error if illegal.
	SetParent(n Node) error

	// Children returns the node's children (which may be empty).
	Children() []Node

	// SetChildren sets the node's children, returning an error if illegal.
	SetChildren(children []Node) error

	// AddChild adds a child node, returning an error if illegal.
	AddChild(child Node) error

	// Context returns the parse tree context.
	Context() antlr.ParseTree

	// SetContext sets the parse tree context, returning an error if illegal.
	SetContext(ctx antlr.ParseTree) error

	// String returns a debug-friendly string representation.
	String() string

	// Text returns the node's text representation.
	Text() string
}

// Selectable is a marker interface to indicate that the node can be
// selected from. That is, the node represents a SQL table, view, or
// join table, and can be used like "SELECT * FROM [selectable]".
type Selectable interface {
	Selectable()
}

// ColExpr indicates a column selection expression such as a
// column name, or context-appropriate function, e.g. "COUNT(*)".
type ColExpr interface {
	// IsColName returns true if the expr is a column name, e.g. "uid" or "users.uid".
	IsColName() bool

	// ColExpr returns the column expression value. For a simple ColSelector ".first_name",
	// this would be "first_name".
	ColExpr() (string, error)

	// String returns a log/debug-friendly representation.
	String() string

	// Alias returns the column alias, which may be empty.
	// For example, given the selector ".first_name:given_name", the alias is "given_name".
	Alias() string
}

// baseNode is a base implementation of Node.
type baseNode struct {
	parent   Node
	children []Node
	ctx      antlr.ParseTree
}

func (bn *baseNode) Parent() Node {
	return bn.parent
}

func (bn *baseNode) SetParent(parent Node) error {
	bn.parent = parent
	return nil
}

func (bn *baseNode) Children() []Node {
	return bn.children
}

func (bn *baseNode) AddChild(child Node) error {
	return errorf(msgNodeNoAddChild, bn, child)
}

func (bn *baseNode) addChild(child Node) {
	bn.children = append(bn.children, child)
}

func (bn *baseNode) SetChildren(children []Node) error {
	return errorf(msgNodeNoAddChildren, bn, len(children))
}

func (bn *baseNode) setChildren(children []Node) {
	bn.children = children
}

func (bn *baseNode) Text() string {
	if bn.ctx == nil {
		return ""
	}

	return bn.ctx.GetText()
}

func (bn *baseNode) Context() antlr.ParseTree {
	return bn.ctx
}

func (bn *baseNode) SetContext(ctx antlr.ParseTree) error {
	bn.ctx = ctx
	return nil
}

// nodeString returns a default value suitable for use by Node.String().
func nodeString(n Node) string {
	return fmt.Sprintf("%T: %s", n, n.Text())
}

// replaceNode replaces old with new. That is, nu becomes a child
// of old's parent.
func replaceNode(old, nu Node) error {
	err := nu.SetContext(old.Context())
	if err != nil {
		return err
	}

	parent := old.Parent()

	index := childIndex(parent, old)
	if index < 0 {
		return errorf("parent %T(%q) does not appear to have child %T(%q)", parent, parent.Text(), old, old.Text())
	}
	siblings := parent.Children()
	siblings[index] = nu

	return parent.SetChildren(siblings)
}

// childIndex returns the index of child in parent's children, or -1.
func childIndex(parent, child Node) int {
	index := -1

	for i, node := range parent.Children() {
		if node == child {
			index = i
			break
		}
	}

	return index
}

// nodesWithType returns a new slice containing each member of nodes that is
// of the specified type.
func nodesWithType(nodes []Node, typ reflect.Type) []Node {
	s := make([]Node, 0)

	for _, n := range nodes {
		if reflect.TypeOf(n) == typ {
			s = append(s, n)
		}
	}
	return s
}

// Terminal is a terminal/leaf node that typically is interpreted simply as its
// text value.
type Terminal struct {
	baseNode
}

func (t *Terminal) String() string {
	return nodeString(t)
}

// Group models GROUP BY.
type Group struct {
	baseNode
}

func (g *Group) AddChild(child Node) error {
	_, ok := child.(*ColSelector)
	if !ok {
		return errorf("GROUP() only accepts children of type %s, but got %T", typeColSelector, child)
	}

	g.addChild(child)
	return child.SetParent(g)
}

func (g *Group) String() string {
	text := nodeString(g)
	return text
}

// Expr models a SLQ expression such as ".uid > 4".
type Expr struct {
	baseNode
}

func (e *Expr) AddChild(child Node) error {
	e.addChild(child)
	return child.SetParent(e)
}

func (e *Expr) String() string {
	text := nodeString(e)
	return text
}

// Operator is a leaf node in an expression representing an operator such as ">" or "==".
type Operator struct {
	baseNode
}

func (o *Operator) String() string {
	return nodeString(o)
}

// Literal is a leaf node representing a literal such as a number or a string.
type Literal struct {
	baseNode
}

func (li *Literal) String() string {
	return nodeString(li)
}

// Where represents a SQL WHERE clause, i.e. a filter on the SELECT.
type Where struct {
	baseNode
}

func (w *Where) String() string {
	return nodeString(w)
}

// Expr returns the expression that constitutes the SetWhere clause, or nil if no expression.
func (w *Where) Expr() *Expr {
	if len(w.children) == 0 {
		return nil
	}

	return w.children[0].(*Expr)
}

func (w *Where) AddChild(node Node) error {
	expr, ok := node.(*Expr)
	if !ok {
		return errorf("WHERE child must be *Expr, but got: %T", node)
	}

	if len(w.children) > 0 {
		return errorf("WHERE has max 1 child: failed to add: %T", node)
	}

	w.addChild(expr)
	return nil
}

// isOperator returns true if the supplied string is a recognized operator, e.g. "!=" or ">".
func isOperator(text string) bool {
	switch text {
	case "-", "+", "~", "!", "||", "*", "/", "%", "<<", ">>", "&", "<", "<=", ">", ">=", "==", "!=", "&&":
		return true
	default:
		return false
	}
}

// Cached results from reflect.TypeOf for node types.
var (
	typeAST         = reflect.TypeOf((*AST)(nil))
	typeDatasource  = reflect.TypeOf((*Datasource)(nil))
	typeSegment     = reflect.TypeOf((*Segment)(nil))
	typeJoin        = reflect.TypeOf((*Join)(nil))
	typeSelector    = reflect.TypeOf((*Selector)(nil))
	typeColSelector = reflect.TypeOf((*ColSelector)(nil))
	typeTblSelector = reflect.TypeOf((*TblSelector)(nil))
	typeRowRange    = reflect.TypeOf((*RowRange)(nil))
	typeExpr        = reflect.TypeOf((*Expr)(nil))
)
