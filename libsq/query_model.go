package libsq

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/loz"
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
	Having   *ast.HavingNode
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
	if err = specifyTableFully(qc, qm.Table); err != nil {
		return nil, err
	}

	if qm.Joins, err = insp.FindJoins(); err != nil {
		return nil, err
	}
	for _, join := range qm.Joins {
		if err = specifyTableFully(qc, join.Table()); err != nil {
			return nil, err
		}
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

	if qm.GroupBy, err = insp.FindGroupByNode(); err != nil {
		return nil, err
	}

	if qm.Having, err = insp.FindHavingNode(); err != nil {
		return nil, err
	}
	if qm.Having != nil && qm.GroupBy == nil {
		return nil, errz.Errorf("having() clause requires preceding group_by() clause")
	}

	if qm.Distinct, err = insp.FindUniqueNode(); err != nil {
		return nil, err
	}
	if qm.OrderBy, err = insp.FindOrderByNode(); err != nil {
		return nil, err
	}

	return qm, nil
}

// specifyTableFully sets the handle, catalog and schema for tbl, if
// possible. If tbl is nil, this is a no-op.
func specifyTableFully(qc *QueryContext, tbl *ast.TblSelectorNode) error {
	if tbl == nil {
		return nil
	}

	if tbl.Handle() == "" {
		// It's possible that there's no active source: this
		// is effectively a no-op in that case.
		tbl.SetHandle(qc.Collection.ActiveHandle())
	}

	if tbl.Handle() != "" {
		// Check if the source has catalog and schema overrides set,
		// and if so, update tbl with those values.
		src, err := qc.Collection.Get(tbl.Handle())
		if err != nil {
			return err
		}

		tfq := tbl.Table()
		if src.Catalog != "" {
			tfq.Catalog = src.Catalog
		}
		if src.Schema != "" {
			tfq.Schema = src.Schema
		}
		tbl.SetTable(tfq)
	}

	return nil
}
