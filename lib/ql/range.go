package ql

import "github.com/neilotoole/sq/lib/ql/parser"

type RowRange struct {
	BaseNode
	offset int
	limit  int
}

func NewRowRange(ctx *parser.RowRangeContext, offset int, limit int) *RowRange {

	rr := &RowRange{}
	rr.ctx = ctx
	rr.offset = offset
	rr.limit = limit
	return rr
}

func (rr *RowRange) String() string {
	return rr.Text()
}

func (rr *RowRange) Range() (offset int, limit int) {

	offset = rr.offset
	limit = rr.limit
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
