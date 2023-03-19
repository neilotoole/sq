package ast

var (
	_ Node    = (*Func)(nil)
	_ ColExpr = (*Func)(nil)
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

// ColExpr implements ColExpr.
func (fn *Func) ColExpr() (string, error) {
	return fn.ctx.GetText(), nil
}

// Alias implements ColExpr.
func (fn *Func) Alias() string {
	return fn.alias
}

// SetChildren implements Node.
func (fn *Func) SetChildren(children []Node) error {
	fn.setChildren(children)
	return nil
}

// IsColName implements ColExpr.
func (fn *Func) IsColName() bool {
	return false
}

func (fn *Func) AddChild(child Node) error {
	// TODO: add check for valid Func child types
	fn.addChild(child)
	return child.SetParent(fn)
}
