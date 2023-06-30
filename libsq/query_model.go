package libsq

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/core/loz"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// queryModel is a model of an SLQ query built from the AST.
type queryModel struct {
	AST      *ast.AST
	Table    *ast.TblSelectorNode
	Joins    []*ast.JoinNode
	Cols     []ast.ResultColumn
	Range    *ast.RowRangeNode
	Where    *ast.WhereNode
	OrderBy  *ast.OrderByNode
	GroupBy  *ast.GroupByNode
	Distinct *ast.UniqueNode
}

func (qm *queryModel) String() string {
	return fmt.Sprintf("%v | %v  |  %v", qm.Table, qm.Cols, qm.Range)
}

// buildQueryModel creates a queryModel instance from the AST.
func buildQueryModel(qc *QueryContext, a *ast.AST) (*queryModel, error) {
	if len(a.Segments()) == 0 {
		return nil, errz.Errorf("invalid query: no segments")
	}

	var (
		ok   bool
		err  error
		insp = ast.NewInspector(a)
		qm   = &queryModel{AST: a}
	)

	qm.Table = insp.FindFirstTableSelector()
	if qm.Table != nil {
		// If the table selector doesn't specify a handle, set the
		// table's handle to the active handle.
		if qm.Table.Handle() == "" {
			// It's possible that there's no active source: this
			// is effectively a no-op in that case.
			qm.Table.SetHandle(qc.Collection.ActiveHandle())
		}
	}

	if qm.Joins, err = insp.FindJoins(); err != nil {
		return nil, err
	}

	if len(qm.Joins) > 0 && qm.Table == nil {
		return nil, errz.Errorf("invalid query: join doesn't have a preceding table selector")
	}

	if qm.Range, err = insp.FindRowRangeNode(); err != nil {
		return nil, err
	}

	seg, err := insp.FindColExprSegment()
	if err != nil {
		return nil, err
	}

	if seg != nil {
		var colExprs []ast.ResultColumn
		if colExprs, ok = loz.ToSliceType[ast.Node, ast.ResultColumn](seg.Children()...); !ok {
			return nil, errz.Errorf("segment children contained elements that were not of type %T: %s",
				ast.ResultColumn(nil), seg.Text())
		}

		qm.Cols = colExprs
	}

	whereClauses, err := insp.FindWhereClauses()
	if err != nil {
		return nil, err
	}

	if len(whereClauses) > 1 {
		return nil, errz.Errorf("only one WHERE clause is supported, but found %d", len(whereClauses))
	} else if len(whereClauses) == 1 {
		qm.Where = whereClauses[0]
	}

	if qm.OrderBy, err = insp.FindOrderByNode(); err != nil {
		return nil, err
	}

	if qm.GroupBy, err = insp.FindGroupByNode(); err != nil {
		return nil, err
	}

	if qm.Distinct, err = insp.FindUniqueNode(); err != nil {
		return nil, err
	}

	return qm, nil
}
