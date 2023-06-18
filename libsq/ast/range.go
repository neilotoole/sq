package ast

import (
	"strconv"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// RowRangeNode models a range, effectively {OFFSET,LIMIT}.
type RowRangeNode struct {
	baseNode
	Offset int
	Limit  int
}

func newRowRangeNode(ctx *slq.RowRangeContext, offset, limit int) *RowRangeNode {
	rr := &RowRangeNode{}
	rr.ctx = ctx
	rr.text = ctx.GetText()
	rr.Offset = offset
	rr.Limit = limit
	return rr
}

// String implements ast.Node.
func (rr *RowRangeNode) String() string {
	return rr.Text()
}

// Range returns the offset and limit.
func (rr *RowRangeNode) Range() (offset, limit int) {
	offset = rr.Offset
	limit = rr.Limit
	return offset, limit
}

// SetParent implements ast.Node.
func (rr *RowRangeNode) SetParent(parent Node) error {
	seg, ok := parent.(*SegmentNode)
	if !ok {
		return errorf("%T requires parent of type *%s", rr, typeSegmentNode)
	}
	rr.parent = seg
	return nil
}

// VisitRowRange implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitRowRange(ctx *slq.RowRangeContext) any {
	// []      select all rows (no range)
	// [1]     select row[1]
	// [10:15] select rows 10 thru 15
	// [0:15]  select rows 0 thru 15
	// [:15]   same as above (0 thru 15)
	// [10:]   select all rows from 10 onwards

	if ctx.COLON() == nil && len(ctx.AllNN()) == 0 {
		// [] select all rows, aka no range
		return nil
	}

	if ctx.COLON() == nil {
		// [1] -- select row[1]
		if len(ctx.AllNN()) != 1 {
			return errorf("row range: expected one integer but got %d", len(ctx.AllNN()))
		}

		i, _ := strconv.Atoi(ctx.AllNN()[0].GetText())
		rr := newRowRangeNode(ctx, i, 1)
		return v.cur.AddChild(rr)
	}

	// there's a colon... can only be one or two ints
	if len(ctx.AllNN()) > 2 {
		return errorf("row range: expected one or two integers but got %d", len(ctx.AllNN()))
	}

	if len(ctx.AllNN()) == 2 {
		// [10:15] -- select rows 10 thru 15
		offset, _ := strconv.Atoi(ctx.AllNN()[0].GetText())
		finish, _ := strconv.Atoi(ctx.AllNN()[1].GetText())
		limit := finish - offset
		rr := newRowRangeNode(ctx, offset, limit)
		return v.cur.AddChild(rr)
	}

	// it's one of these two cases:
	//   [:15]   (0 thru 15)
	//   [10:]   select all rows from 10 onwards
	// so we need to determine if the INT is before or after the colon
	var offset int
	limit := -1

	if ctx.COLON().GetSymbol().GetTokenIndex() < ctx.AllNN()[0].GetSymbol().GetTokenIndex() {
		// [:15]   (0 thru 15)
		offset = 0
		limit, _ = strconv.Atoi(ctx.AllNN()[0].GetText())
	} else {
		// [10:]   select all rows from 10 onwards
		offset, _ = strconv.Atoi(ctx.AllNN()[0].GetText())
	}

	rr := newRowRangeNode(ctx, offset, limit)
	return v.cur.AddChild(rr)
}

// verifyRowRange validates the RowRangeNode element.
func verifyRowRange(w *Walker, node Node) error {
	rr, ok := node.(*RowRangeNode)
	if !ok {
		return errorf("expected %s but got %T", typeRowRangeNode, node)
	}

	if w.state != nil {
		return errorf("only one row range element permitted")
	}

	w.state = rr
	// TODO: check that the row range is after a selectable
	return nil
}
