package engine

import (
	"fmt"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/drvr"
)

// Plan models a sq execution plan.
// At this time Plan effectively models a query, but in future it will also model
// operations such as copy/inserts etc.
type Plan struct {
	AST        *ast.AST
	Selectable ast.Selectable
	Cols       []ast.ColExpr
	Range      *ast.RowRange
}

func (s *Plan) String() string {
	return fmt.Sprintf("%v | %v  |  %v", s.Selectable, s.Cols, s.Range)
}

// PlanBuilder builds an execution plan from an AST.
type PlanBuilder struct {
	// srcs is not currently used by the builder, but in the future it will
	// be used to provide more intelligence in the build/error-checking process.
	srcs *drvr.SourceSet
}

// NewPlanBuilder returns a new plan builder instance.
func NewPlanBuilder(srcs *drvr.SourceSet) *PlanBuilder {
	return &PlanBuilder{srcs: srcs}
}

// Model builds a SelectStmt from the IR.
func (pb *PlanBuilder) Build(a *ast.AST) (*Plan, error) {

	lg.Debugf("starting to build model")

	lg.Debugf("starting build2()")
	if len(a.Segments()) == 0 {
		return nil, errorf("parse error: the query does not have enough segments")
	}

	stmt := &Plan{AST: a}

	insp := ast.NewInspector(a)
	selectableSeg, err := insp.FindFinalSelectableSegment()
	if err != nil {
		return nil, err
	}

	if len(selectableSeg.Children()) != 1 {
		return nil, errorf("the final selectable segment must have exactly one selectable element, but found %d elements", len(selectableSeg.Children()))
	}

	selectable, ok := selectableSeg.Children()[0].(ast.Selectable)
	if !ok {
		return nil, errorf("the final selectable segment must have exactly one selectable element, but found element %T(%q)", selectableSeg.Children()[0], selectableSeg.Children()[0].Text())
	}

	stmt.Selectable = selectable
	lg.Debugf("found selectable segment: %q", selectableSeg.Text())

	// Look for range
	for seg := selectableSeg.Next(); seg != nil; seg = seg.Next() {

		childType, err := seg.ChildType()
		if err != nil {
			return nil, err
		}

		if childType == ast.TypeRowRange {
			if len(seg.Children()) != 1 {
				return nil, errorf("segment [%d] with row range must have exactly one element, but found %d: %q", seg.SegIndex(), len(seg.Children()), seg.Text())
			}

			rr, ok := seg.Children()[0].(*ast.RowRange)
			if !ok {
				return nil, errorf("expected row range, but got %T(%q)", seg.Children()[0], seg.Children()[0].Text())
			}

			if stmt.Range != nil {
				return nil, errorf("only one row range permitted, but found %q and %q", stmt.Range.Text(), rr.Text())
			}

			lg.Debugf("found row range: %q", rr.Text())
			stmt.Range = rr
		}
	}

	seg, err := insp.FindColExprSegment()
	if err != nil {
		return nil, err
	}

	if seg == nil {
		lg.Debugf("did not find a col expr segment")
	} else {
		lg.Debugf("found col expr segment: %s", seg.Text())
		elems := seg.Children()
		colExprs := make([]ast.ColExpr, len(elems))
		for i, elem := range elems {
			colExpr, ok := elem.(ast.ColExpr)
			if !ok {
				return nil, errorf("expected element in segment [%d] to be col expr, but was %T", i, elem)
			}

			colExprs[i] = colExpr
		}

		stmt.Cols = colExprs
	}

	return stmt, nil
}
