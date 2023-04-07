package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/ast/render"

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
		err   error
		frags = &render.Fragments{}
	)

	// After this switch, ng.bc will be set.
	switch node := qm.Table.(type) {
	case *ast.TblSelectorNode:
		if frags.From, ng.targetDB, err = ng.buildTableFromClause(ctx, node); err != nil {
			return err
		}
	case *ast.JoinNode:
		if frags.From, ng.targetDB, err = ng.buildJoinFromClause(ctx, node); err != nil {
			return err
		}
	default:
		// Should never happen
		return errz.Errorf("unknown selectable %T: %s", node, node)
	}

	rndr := ng.targetDB.SQLDriver().Renderer()

	if frags.Columns, err = rndr.SelectCols(ng.bc, rndr, qm.Cols); err != nil {
		return err
	}

	if qm.Distinct != nil {
		if frags.Distinct, err = rndr.Distinct(ng.bc, rndr, qm.Distinct); err != nil {
			return err
		}
	}

	if qm.Range != nil {
		if frags.Range, err = rndr.Range(ng.bc, rndr, qm.Range); err != nil {
			return err
		}
	}

	if qm.Where != nil {
		if frags.Where, err = rndr.Where(ng.bc, rndr, qm.Where); err != nil {
			return err
		}
	}

	if qm.OrderBy != nil {
		if frags.OrderBy, err = rndr.OrderBy(ng.bc, rndr, qm.OrderBy); err != nil {
			return err
		}
	}

	if qm.GroupBy != nil {
		if frags.GroupBy, err = rndr.GroupBy(ng.bc, rndr, qm.GroupBy); err != nil {
			return err
		}
	}

	if rndr.PreRender != nil {
		if err = rndr.PreRender(ng.bc, rndr, frags); err != nil {
			return err
		}
	}

	ng.targetSQL, err = rndr.Render(ng.bc, rndr, frags)
	return err
}
