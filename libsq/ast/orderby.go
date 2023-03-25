package ast

// OrderByNode implements the SQL "ORDER BY" clause.
type OrderByNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *OrderByNode) String() string {
	return nodeString(n)
}

// Terms returns a new slice containing the ordering terms of the OrderByNode.
// The returned slice has at least one term.
func (n *OrderByNode) Terms() []*OrderByTermNode {
	terms := make([]*OrderByTermNode, len(n.children))
	for i := range n.children {
		terms[i] = n.children[i].(*OrderByTermNode)
	}
	return terms
}

// AddChild implements Node.AddChild. It returns an error
// if child is nil or not type OrderByTermNode.
func (n *OrderByNode) AddChild(child Node) error {
	term, ok := child.(*OrderByTermNode)
	if !ok {
		return errorf("can only add child of type %T to %T but got %T", term, n, child)
	}

	n.addChild(child)
	return nil
}

// OrderByDirection specifies the "ORDER BY" direction.
type OrderByDirection string

const (
	// OrderByDirectionNone is the default order direction.
	OrderByDirectionNone OrderByDirection = ""

	// OrderByDirectionAsc is the ascending  (DESC) order direction.
	OrderByDirectionAsc OrderByDirection = "ASC"

	// OrderByDirectionDesc is the descending (DESC) order direction.
	OrderByDirectionDesc OrderByDirection = "DESC"
)

// OrderByTermNode is a child of OrderByNode.
type OrderByTermNode struct {
	baseNode
	selector  *SelectorNode
	direction OrderByDirection
}

// Selector returns the ordering term's selector.
func (n *OrderByTermNode) Selector() *SelectorNode {
	return n.selector
}

// Direction returns the ordering term's direction.
func (n *OrderByTermNode) Direction() OrderByDirection {
	return n.direction
}

// String returns a log/debug-friendly representation.
func (n *OrderByTermNode) String() string {
	return nodeString(n)
}
