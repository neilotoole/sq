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
//
// REVISIT: the name "Selectable" might be confusing. Perhaps "Tabler" or such.
type Selectable interface {
	Node
	selectable()
}

// selector is a marker interface for selector types.
type selector interface {
	Node
	selector()
}

// ResultColumn indicates a column selection expression such as a
// column name, or context-appropriate function, e.g. "COUNT(*)".
// See: https://www.sqlite.org/syntax/result-column.html
type ResultColumn interface {
	// IsColumn returns true if the expression represents
	// a column, e.g. ".first_name" or "actor.first_name".
	// This method returns false for functions, e.g. "COUNT(*)".
	// REVISIT: We can probably get rid of this?
	IsColumn() bool

	// String returns a log/debug-friendly representation.
	String() string

	// Alias returns the column alias, which may be empty.
	// For example, given the selector ".first_name:given_name", the
	// alias is "given_name".
	Alias() string

	// Text returns the raw text of the node, e.g. ".actor" or "1*2".
	Text() string
}

// baseNode is a base implementation of Node.
type baseNode struct {
	parent   Node
	children []Node
	ctx      antlr.ParseTree
	text     string
}

// Parent implements Node.Parent.
func (bn *baseNode) Parent() Node {
	return bn.parent
}

// SetParent implements Node.SetParent.
func (bn *baseNode) SetParent(parent Node) error {
	bn.parent = parent
	return nil
}

// Children implements Node.Children.
func (bn *baseNode) Children() []Node {
	return bn.children
}

// AddChild always returns an error. Node implementations should
// implement a type-specific method that only accepts a child of
// an appropriate type for that node.
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

// nodeReplace replaces old with new. That is, nu becomes a child
// of old's parent.
func nodeReplace(old, nu Node) error {
	err := nu.SetContext(old.Context())
	if err != nil {
		return err
	}

	parent := old.Parent()

	index := nodeChildIndex(parent, old)
	if index < 0 {
		return errorf("parent %T(%q) does not appear to have child %T(%q)", parent, parent.Text(), old, old.Text())
	}
	siblings := parent.Children()
	siblings[index] = nu

	return parent.SetChildren(siblings)
}

// nodeChildIndex returns the index of child in parent's children, or -1.
func nodeChildIndex(parent, child Node) int {
	for i, node := range parent.Children() {
		if node == child {
			return i
		}
	}

	return -1
}

// nodeFirstChild returns the first child of parent, or nil.
func nodeFirstChild(parent Node) Node { //nolint:unused
	if parent == nil {
		return nil
	}

	children := parent.Children()
	if len(children) == 0 {
		return nil
	}

	return children[0]
}

// nodeFirstChild returns the last child of parent, or nil.
func nodeLastChild(parent Node) Node {
	if parent == nil {
		return nil
	}

	children := parent.Children()
	if len(children) == 0 {
		return nil
	}

	return children[len(children)-1]
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

// GroupByNode models GROUP BY.
type GroupByNode struct {
	baseNode
}

// AddChild implements Node.
func (n *GroupByNode) AddChild(child Node) error {
	_, ok := child.(selector)
	if !ok {
		return errorf("GROUP() only accepts children of type %s, but got %T", typeColSelectorNode, child)
	}

	n.addChild(child)
	return child.SetParent(n)
}

// SetChildren implements ast.Node.
func (n *GroupByNode) SetChildren(children []Node) error {
	for i := range children {
		if _, ok := children[i].(selector); !ok {
			return errorf("illegal child [%d] type %T {%s} for %T", i, children[i], children[i], n)
		}
	}

	n.setChildren(children)
	return nil
}

// String returns a log/debug-friendly representation.
func (n *GroupByNode) String() string {
	text := nodeString(n)
	return text
}

// ExprNode models a SLQ expression such as ".uid > 4".
type ExprNode struct {
	baseNode
}

// AddChild implements Node.
func (n *ExprNode) AddChild(child Node) error {
	n.addChild(child)
	return child.SetParent(n)
}

// SetChildren implements Node.
func (n *ExprNode) SetChildren(children []Node) error {
	n.setChildren(children)
	return nil
}

// String returns a log/debug-friendly representation.
func (n *ExprNode) String() string {
	text := nodeString(n)
	return text
}

// OperatorNode is a leaf node in an expression representing an operator such as ">" or "==".
type OperatorNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *OperatorNode) String() string {
	return nodeString(n)
}

// LiteralNode is a leaf node representing a literal such as a number or a string.
type LiteralNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *LiteralNode) String() string {
	return nodeString(n)
}

// WhereNode represents a SQL WHERE clause, i.e. a filter on the SELECT.
type WhereNode struct {
	baseNode
}

func (n *WhereNode) String() string {
	return nodeString(n)
}

// Expr returns the expression that constitutes the SetWhere clause, or nil if no expression.
func (n *WhereNode) Expr() *ExprNode {
	if len(n.children) == 0 {
		return nil
	}

	return n.children[0].(*ExprNode)
}

func (n *WhereNode) AddChild(node Node) error {
	expr, ok := node.(*ExprNode)
	if !ok {
		return errorf("WHERE child must be %T, but got: %T", expr, node)
	}

	if len(n.children) > 0 {
		return errorf("WHERE has max 1 child: failed to add: %T", node)
	}

	n.addChild(expr)
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
	typeAST             = reflect.TypeOf((*AST)(nil))
	typeHandleNode      = reflect.TypeOf((*HandleNode)(nil))
	typeSegmentNode     = reflect.TypeOf((*SegmentNode)(nil))
	typeJoinNode        = reflect.TypeOf((*JoinNode)(nil))
	typeSelectorNode    = reflect.TypeOf((*SelectorNode)(nil))
	typeColSelectorNode = reflect.TypeOf((*ColSelectorNode)(nil))
	typeTblSelectorNode = reflect.TypeOf((*TblSelectorNode)(nil))
	typeRowRangeNode    = reflect.TypeOf((*RowRangeNode)(nil))
	typeOrderByNode     = reflect.TypeOf((*OrderByNode)(nil))
	typeGroupByNode     = reflect.TypeOf((*GroupByNode)(nil))
	typeExprNode        = reflect.TypeOf((*ExprNode)(nil))
)
