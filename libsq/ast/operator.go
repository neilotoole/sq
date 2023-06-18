package ast

// OperatorNode is a leaf node in an expression representing an operator such as ">" or "==".
type OperatorNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *OperatorNode) String() string {
	return nodeString(n)
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
