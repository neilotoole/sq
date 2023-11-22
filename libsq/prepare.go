package libsq

import (
	"context"

	"github.com/neilotoole/sq/libsq/ast/render"
)

// prepare prepares the pipeline to execute queryModel.
// When this method returns, targetPool and targetSQL will be set,
// as will any tasks (which may be empty). The tasks must be executed
// against targetPool before targetSQL is executed (the pipeline.execute
// method does this work).
func (p *pipeline) prepare(ctx context.Context, qm *queryModel) error {
	var err error

	frags := &render.Fragments{}

	// After this switch, p.rc will be set.
	switch {
	case qm.Table == nil:
		if err = p.prepareNoTable(ctx, qm); err != nil {
			return err
		}
	case len(qm.Joins) > 0:
		jc := &joinClause{leftTbl: qm.Table, joins: qm.Joins}
		if frags.From, p.targetPool, err = p.prepareFromJoin(ctx, jc); err != nil {
			return err
		}
	default:
		if frags.From, p.targetPool, err = p.prepareFromTable(ctx, qm.Table); err != nil {
			return err
		}
	}

	p.rc.Fragments = frags
	rndr := p.rc.Renderer
	if frags.Columns, err = rndr.SelectCols(p.rc, qm.Cols); err != nil {
		return err
	}

	if qm.Distinct != nil {
		if frags.Distinct, err = rndr.Distinct(p.rc, qm.Distinct); err != nil {
			return err
		}
	}

	if qm.Range != nil {
		if frags.Range, err = rndr.Range(p.rc, qm.Range); err != nil {
			return err
		}
	}

	if qm.Where != nil {
		if frags.Where, err = rndr.Where(p.rc, qm.Where); err != nil {
			return err
		}
	}

	if qm.OrderBy != nil {
		if frags.OrderBy, err = rndr.OrderBy(p.rc, qm.OrderBy); err != nil {
			return err
		}
	}

	if qm.GroupBy != nil {
		if frags.GroupBy, err = rndr.GroupBy(p.rc, qm.GroupBy); err != nil {
			return err
		}
	}

	if qm.Having != nil {
		if frags.Having, err = rndr.Having(p.rc, qm.Having); err != nil {
			return err
		}
	}

	for _, fn := range rndr.PreRender {
		if err = fn(p.rc, frags); err != nil {
			return err
		}
	}

	if p.targetSQL, err = rndr.Render(p.rc, frags); err != nil {
		return err
	}

	p.qc.PreExecStmts = append(p.qc.PreExecStmts, frags.PreExecStmts...)
	p.qc.PostExecStmts = append(p.qc.PostExecStmts, frags.PostExecStmts...)

	return nil
}
