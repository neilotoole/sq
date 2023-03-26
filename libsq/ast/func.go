package ast

var (
	_ Node         = (*FuncNode)(nil)
	_ ResultColumn = (*FuncNode)(nil)
)

// FuncNode models a function. For example, "COUNT()".
type FuncNode struct {
	baseNode
	fnName string
	alias  string
}

// FuncName returns the function name.
func (fn *FuncNode) FuncName() string {
	return fn.fnName
}

// String returns a log/debug-friendly representation.
func (fn *FuncNode) String() string {
	str := nodeString(fn)
	if fn.alias != "" {
		str += ":" + fn.alias
	}
	return str
}

// Text implements ResultColumn.
func (fn *FuncNode) Text() string {
	return fn.ctx.GetText()
}

// Alias implements ResultColumn.
func (fn *FuncNode) Alias() string {
	return fn.alias
}

// SetChildren implements Node.
func (fn *FuncNode) SetChildren(children []Node) error {
	fn.setChildren(children)
	return nil
}

// IsColumn implements ResultColumn.
func (fn *FuncNode) IsColumn() bool {
	return false
}

func (fn *FuncNode) AddChild(child Node) error {
	// TODO: add check for valid FuncNode child types
	fn.addChild(child)
	return child.SetParent(fn)
}
