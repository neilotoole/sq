package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// prepare prepares the engine to execute queryModel.
// When this method returns, targetDB and targetSQL will be set,
// as will any tasks (which may be empty). The tasks must be executed
// against targetDB before targetSQL is executed (the engine.execute
// method does this work).
func (ng *engine) prepare(ctx context.Context, qm *queryModel) error {
	var (
		s   string
		err error
	)

	// After this switch, ng.bc will be set.
	switch node := qm.Table.(type) {
	case *ast.TblSelectorNode:
		s, ng.targetDB, err = ng.buildTableFromClause(ctx, node)
		if err != nil {
			return err
		}
	case *ast.JoinNode:
		s, ng.targetDB, err = ng.buildJoinFromClause(ctx, node)
		if err != nil {
			return err
		}
	default:
		// Should never happen
		return errz.Errorf("unknown selectable %T: %s", node, node)
	}

	rndr, qb := ng.targetDB.SQLDriver().SQLBuilder()
	qb.SetFrom(s)

	if s, err = rndr.SelectCols(ng.bc, rndr, qm.Cols); err != nil {
		return err
	}
	qb.SetColumns(s)

	if qm.Distinct != nil {
		if s, err = rndr.Distinct(ng.bc, rndr, qm.Distinct); err != nil {
			return err
		}

		qb.SetDistinct(s)
	}

	if qm.Range != nil {
		if s, err = rndr.Range(ng.bc, rndr, qm.Range); err != nil {
			return err
		}

		qb.SetRange(s)
	}

	if qm.Where != nil {
		if s, err = rndr.Where(ng.bc, rndr, qm.Where); err != nil {
			return err
		}

		qb.SetWhere(s)
	}

	if qm.OrderBy != nil {
		if s, err = rndr.OrderBy(ng.bc, rndr, qm.OrderBy); err != nil {
			return err
		}

		qb.SetOrderBy(s)
	}

	if qm.GroupBy != nil {
		if s, err = rndr.GroupBy(ng.bc, rndr, qm.GroupBy); err != nil {
			return err
		}
		qb.SetGroupBy(s)
	}

	ng.targetSQL, err = qb.Render()
	if err != nil {
		return err
	}

	return nil
}
