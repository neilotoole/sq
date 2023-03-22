package ast

var (
	_ Node         = (*Func)(nil)
	_ ResultColumn = (*Func)(nil)
)

// Func models a function. For example, "COUNT()".
type Func struct {
	baseNode
	fnName string
	alias  string
}

// FuncName returns the function name.
func (fn *Func) FuncName() string {
	return fn.fnName
}

// String returns a log/debug-friendly representation.
func (fn *Func) String() string {
	str := nodeString(fn)
	if fn.alias != "" {
		str += ":" + fn.alias
	}
	return str
}

// Text implements ResultColumn.
func (fn *Func) Text() string {
	return fn.ctx.GetText()
}

// Alias implements ResultColumn.
func (fn *Func) Alias() string {
	return fn.alias
}

// SetChildren implements Node.
func (fn *Func) SetChildren(children []Node) error {
	fn.setChildren(children)
	return nil
}

// IsColumn implements ResultColumn.
func (fn *Func) IsColumn() bool {
	return false
}

func (fn *Func) AddChild(child Node) error {
	// TODO: add check for valid Func child types
	fn.addChild(child)
	return child.SetParent(fn)
}
