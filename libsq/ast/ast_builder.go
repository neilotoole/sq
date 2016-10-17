package ast

import (
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/slq"
)

// NewBuilder returns a new AST builder.
func NewBuilder(srcs *drvr.SourceSet) *Builder {
	return &Builder{srcs}
}

// Builder constructs sq's AST from a parse tree.
type Builder struct {
	// srcs is not currently used by Builder, but in the future it will
	// be used to provide more intelligence in the build/error-checking process.
	srcs *drvr.SourceSet
}

func (ab *Builder) Build(query slq.IQueryContext) (*AST, error) {

	q, ok := query.(*slq.QueryContext)
	if !ok {
		return nil, errorf("unable to convert %T to *parser.QueryContext", query)
	}

	v := &ParseTreeVisitor{}
	q.Accept(v)
	if v.Err != nil {
		return nil, v.Err
	}

	lg.Debugf("After ParseTreeVisitor:\n%s", sprintNodeTree(v.ast))

	insp := NewInspector(v.ast)
	ds, err := insp.FindDataSource()
	if err != nil {
		return nil, err
	}
	v.ast.Datasource = ds

	err = NewWalker(v.ast).AddVisitor(TypeSelector, narrowTblSel).Walk()
	if err != nil {
		return nil, err
	}
	lg.Debugf("After narrowTblSel:\n%s", sprintNodeTree(v.ast))

	err = NewWalker(v.ast).AddVisitor(TypeSelector, narrowColSel).Walk()
	if err != nil {
		return nil, err
	}
	lg.Debugf("After narrowColSel:\n%s", sprintNodeTree(v.ast))

	err = NewWalker(v.ast).AddVisitor(TypeFnJoin, determineJoinTables).Walk()
	if err != nil {
		return nil, err
	}

	err = NewWalker(v.ast).AddVisitor(TypeRowRange, visitCheckRowRange).Walk()
	if err != nil {
		return nil, err
	}

	lg.Debugf("After AST visitors:\n%s", sprintNodeTree(v.ast))

	return v.ast, nil

}
