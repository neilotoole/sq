package libsq

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/core/loz"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"golang.org/x/exp/slog"
)

// queryModel is a model of an SLQ query built from the AST.
type queryModel struct {
	AST      *ast.AST
	Table    ast.Tabler
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
func buildQueryModel(log *slog.Logger, a *ast.AST) (*queryModel, error) {
	if len(a.Segments()) == 0 {
		return nil, errz.Errorf("query model error: query does not have enough segments")
	}

	insp := ast.NewInspector(a)

	var tabler ast.Tabler
	var ok bool
	tablerSeg, err := insp.FindFinalTablerSegment()
	if err != nil {
		log.Debug("No Tabler segment.")
	}

	if tablerSeg != nil {
		if len(tablerSeg.Children()) != 1 {
			return nil, errz.Errorf(
				"the final selectable segment must have exactly one selectable element, but found %d elements",
				len(tablerSeg.Children()))
		}

		if tabler, ok = tablerSeg.Children()[0].(ast.Tabler); !ok {
			return nil, errz.Errorf(
				"the final selectable segment must have exactly one selectable element, but found element %T(%s)",
				tablerSeg.Children()[0], tablerSeg.Children()[0].Text())
		}
	}

	qm := &queryModel{AST: a, Table: tabler}

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
