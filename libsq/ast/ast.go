// Package ast holds types and functionality for the SLQ AST.
//
// The entrypoint is ast.Parse, which accepts a SLQ query, and
// returns an *ast.AST. The ast is a tree of ast.Node instances.
//
// Note that much of the testing of package ast is performed in
// package libsq.
package ast

import (
	"log/slog"
	"reflect"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
)

// Parse parses the SLQ input string and builds the AST.
func Parse(log *slog.Logger, input string) (*AST, error) { //nolint:staticcheck
	// REVISIT: We need a better solution for disabling parser logging.
	log = lg.Discard() //nolint:staticcheck // Disable parser logging.
	ptree, err := parseSLQ(log, input)
	if err != nil {
		return nil, err
	}

	ast, err := buildAST(log, ptree)
	if err != nil {
		return nil, err
	}

	if err = verify(ast); err != nil {
		return nil, err
	}

	return ast, nil
}

// buildAST constructs sq's AST from a parse tree.
func buildAST(log *slog.Logger, query slq.IQueryContext) (*AST, error) {
	if query == nil {
		return nil, errorf("query is nil")
	}

	q, ok := query.(*slq.QueryContext)
	if !ok {
		return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	}

	tree := &parseTreeVisitor{log: log}

	if err := q.Accept(tree); err != nil {
		return nil, err.(error)
	}

	visitors := []struct {
		typ reflect.Type
		fn  nodeVisitorFn
	}{
		{typeSelectorNode, narrowTblSel},
		{typeSelectorNode, narrowTblColSel},
		{typeSelectorNode, narrowColSel},
		{typeRowRangeNode, verifyRowRange},
	}

	for _, visitor := range visitors {
		w := NewWalker(tree.ast).AddVisitor(visitor.typ, visitor.fn)
		if err := w.Walk(); err != nil {
			return nil, err
		}
	}

	return tree.ast, nil
}

// verify performs additional checks on the state of the built AST.
func verify(ast *AST) error {
	selCount := NewInspector(ast).CountNodes(typeSelectorNode)
	if selCount != 0 {
		return errorf("AST should have zero nodes of type %T but found %d",
			(*SelectorNode)(nil), selCount)
	}

	// TODO: Lots more checks could go here

	return nil
}

var _ Node = (*AST)(nil)

// AST is the Abstract Syntax Tree. It is the root node of a SQL query/stmt.
type AST struct {
	ctx  *slq.QueryContext
	text string
	segs []*SegmentNode
}

// ast implements ast.Node.
func (a *AST) ast() *AST {
	return a
}

// Parent implements ast.Node.
func (a *AST) Parent() Node {
	return nil
}

// SetParent implements ast.Node.
func (a *AST) SetParent(parent Node) error {
	return errorf("root node (%T) cannot have parent: tried to add parent %T", a, parent)
}

// Children implements ast.Node.
func (a *AST) Children() []Node {
	nodes := make([]Node, len(a.segs))

	for i, seg := range a.segs {
		nodes[i] = seg
	}

	return nodes
}

// Segments returns the AST's segments (its direct children).
func (a *AST) Segments() []*SegmentNode {
	return a.segs
}

// AddChild implements ast.Node.
func (a *AST) AddChild(node Node) error {
	seg, ok := node.(*SegmentNode)
	if !ok {
		return errorf("expected *SegmentNode but got: %T", node)
	}

	a.AddSegment(seg)
	return nil
}

// SetChildren implements ast.Node.
func (a *AST) SetChildren(children []Node) error {
	segs := make([]*SegmentNode, len(children))

	for i, child := range children {
		seg, ok := child.(*SegmentNode)
		if !ok {
			return errorf("expected child of type %s, but got: %T", typeSegmentNode, child)
		}

		segs[i] = seg
	}

	a.segs = segs
	return nil
}

// context implements ast.Node.
func (a *AST) context() antlr.ParseTree {
	return a.ctx
}

// setContext implements ast.Node.
func (a *AST) setContext(ctx antlr.ParseTree) error {
	qCtx, ok := ctx.(*slq.QueryContext)
	if !ok {
		return errorf("expected *parser.QueryContext, but got %T", ctx)
	}

	a.ctx = qCtx
	return nil
}

// String implements ast.Node.
func (a *AST) String() string {
	return nodeString(a)
}

// Text implements ast.Node.
func (a *AST) Text() string {
	return a.ctx.GetText()
}

// AddSegment appends seg to the AST.
func (a *AST) AddSegment(seg *SegmentNode) {
	_ = seg.SetParent(a)
	a.segs = append(a.segs, seg)
}

// errorf builds an error. Error creation for this package
// was centralized here in the expectation that an AST-specific
// error type (annotated appropriately) would be returned.
func errorf(format string, v ...any) error {
	return errz.Errorf(format, v...)
}

// ParseCatalogSchema parses a string of the form 'catalog.schema'
// and returns the catalog and schema. It is permissible for one of the
// components to be empty (but not both). Whitespace and quotes are handled
// correctly.
//
// Examples:
//
//	`catalog.schema` 						-> "catalog", "schema", nil
//	`catalog.` 									-> "catalog", "", nil
//	`schema`										-> "", "schema", nil
//	`"my catalog"."my schema"` 	-> "my catalog", "my schema", nil
//
// An error is returned if s is empty.
func ParseCatalogSchema(s string) (catalog, schema string, err error) {
	const errTpl = `invalid catalog.schema: %s`

	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", errz.New("catalog.schema is empty")
	}

	// We'll hijack the existing parser code. A value "catalog.schema" is
	// not valid, but ".catalog.schema" works as a selector.
	//
	// Being that we accept "catalog." as valid (indicating the default schema),
	// we'll use a hack to make the parser work: we append a const string to
	// the input (which the parser will think is the schema name), and later
	// on we check for that const string, and set schema to empty string if
	// that const is found.
	const schemaNameHack = "DEFAULT_SCHEMA_HACK_be8hx64wd45vxusdebez2e6tega8ussy"
	sel := "." + s
	if strings.HasSuffix(s, ".") {
		sel += schemaNameHack
	}

	a, err := Parse(lg.Discard(), sel)
	if err != nil {
		return "", "", errz.Errorf(errTpl, s)
	}

	if len(a.Segments()) != 1 {
		return "", "", errz.Errorf(errTpl, s)
	}

	insp := NewInspector(a)

	tblSel := insp.FindFirstTableSelector()
	if tblSel == nil {
		return "", "", errz.Errorf(errTpl, s)
	}

	if tblSel.name1 == "" {
		schema = tblSel.name0
	} else {
		catalog = tblSel.SelectorNode.name0
		schema = tblSel.SelectorNode.name1
	}
	if schema == "" {
		return "", "", errz.Errorf(errTpl, s)
	} else if schema == schemaNameHack {
		schema = ""
	}

	return catalog, schema, nil
}
