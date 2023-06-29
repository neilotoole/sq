package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/ast/render"
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

	// After this switch, ng.rc will be set.
	switch {
	case qm.Table == nil:
		if err = ng.prepareNoTabler(ctx, qm); err != nil {
			return err
		}
	case len(qm.Joins) > 0:
		jc := &joinClause{leftTbl: qm.Table, joins: qm.Joins}
		if frags.From, ng.targetDB, err = ng.prepareFromJoin(ctx, jc); err != nil {
			return err
		}
	default:
		if frags.From, ng.targetDB, err = ng.prepareFromTable(ctx, qm.Table); err != nil {
			return err
		}
	}

	//
	//switch node := qm.Table.(type) {
	//case nil:
	//
	//case *ast.TblSelectorNode:
	//
	//case *ast.JoinNode:
	//
	//default:
	//	// Should never happen
	//	return errz.Errorf("unknown ast.Tabler %T: %s", node, node)
	//}

	rndr := ng.rc.Renderer

	if frags.Columns, err = rndr.SelectCols(ng.rc, qm.Cols); err != nil {
		return err
	}

	if qm.Distinct != nil {
		if frags.Distinct, err = rndr.Distinct(ng.rc, qm.Distinct); err != nil {
			return err
		}
	}

	if qm.Range != nil {
		if frags.Range, err = rndr.Range(ng.rc, qm.Range); err != nil {
			return err
		}
	}

	if qm.Where != nil {
		if frags.Where, err = rndr.Where(ng.rc, qm.Where); err != nil {
			return err
		}
	}

	if qm.OrderBy != nil {
		if frags.OrderBy, err = rndr.OrderBy(ng.rc, qm.OrderBy); err != nil {
			return err
		}
	}

	if qm.GroupBy != nil {
		if frags.GroupBy, err = rndr.GroupBy(ng.rc, qm.GroupBy); err != nil {
			return err
		}
	}

	if rndr.PreRender != nil {
		if err = rndr.PreRender(ng.rc, frags); err != nil {
			return err
		}
	}

	ng.targetSQL, err = rndr.Render(ng.rc, frags)
	return err
}
