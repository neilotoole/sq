package ast

import "github.com/neilotoole/sq/libsq/slq"

type RowRange struct {
	BaseNode
	Offset int
	Limit  int
}

func NewRowRange(ctx *slq.RowRangeContext, offset int, limit int) *RowRange {

	rr := &RowRange{}
	rr.ctx = ctx
	rr.Offset = offset
	rr.Limit = limit
	return rr
}

func (rr *RowRange) String() string {
	return rr.Text()
}

func (rr *RowRange) Range() (offset int, limit int) {

	offset = rr.Offset
	limit = rr.Limit
	return
}

func (rr *RowRange) SetParent(parent Node) error {

	seg, ok := parent.(*Segment)
	if !ok {
		return errorf("%T requires parent of type *%s", rr, TypeSegment)
	}
	rr.parent = seg
	return nil
}
