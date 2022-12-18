package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

// RowRange models a range, effectively {OFFSET,LIMIT}.
type RowRange struct {
	baseNode
	Offset int
	Limit  int
}

func newRowRange(ctx *slq.RowRangeContext, offset, limit int) *RowRange {
	rr := &RowRange{}
	rr.ctx = ctx
	rr.Offset = offset
	rr.Limit = limit
	return rr
}

func (rr *RowRange) String() string {
	return rr.Text()
}

func (rr *RowRange) Range() (offset, limit int) {
	offset = rr.Offset
	limit = rr.Limit
	return offset, limit
}

func (rr *RowRange) SetParent(parent Node) error {
	seg, ok := parent.(*Segment)
	if !ok {
		return errorf("%T requires parent of type *%s", rr, typeSegment)
	}
	rr.parent = seg
	return nil
}
