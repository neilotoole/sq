package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

// RowRangeNode models a range, effectively {OFFSET,LIMIT}.
type RowRangeNode struct {
	baseNode
	Offset int
	Limit  int
}

func newRowRange(ctx *slq.RowRangeContext, offset, limit int) *RowRangeNode {
	rr := &RowRangeNode{}
	rr.ctx = ctx
	rr.Offset = offset
	rr.Limit = limit
	return rr
}

func (rr *RowRangeNode) String() string {
	return rr.Text()
}

func (rr *RowRangeNode) Range() (offset, limit int) {
	offset = rr.Offset
	limit = rr.Limit
	return offset, limit
}

func (rr *RowRangeNode) SetParent(parent Node) error {
	seg, ok := parent.(*SegmentNode)
	if !ok {
		return errorf("%T requires parent of type *%s", rr, typeSegmentNode)
	}
	rr.parent = seg
	return nil
}
