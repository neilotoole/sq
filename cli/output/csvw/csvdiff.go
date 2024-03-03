package csvw

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/record"
)

func NewDiffWriter(pr *output.Printing) *DiffWriter {
	pr = pr.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false

	dw := &DiffWriter{pr: pr}
	seq := colorz.ExtractSeqs(pr.Diff.Context)
	dw.contextPrefix = slices.Clip(append(seq.Prefix, ' '))
	dw.contextSuffix = slices.Clip(append(seq.Suffix, '\n'))
	seq = colorz.ExtractSeqs(pr.Diff.Insertion)
	dw.insertPrefix = slices.Clip(append(seq.Prefix, '+'))
	dw.insertSuffix = slices.Clip(append(seq.Suffix, '\n'))
	seq = colorz.ExtractSeqs(pr.Diff.Deletion)
	dw.deletePrefix = slices.Clip(append(seq.Prefix, '-'))
	dw.deleteSuffix = slices.Clip(append(seq.Suffix, '\n'))

	return dw
}

type DiffWriter struct {
	pr            *output.Printing
	contextPrefix []byte
	contextSuffix []byte
	insertPrefix  []byte
	insertSuffix  []byte
	deletePrefix  []byte
	deleteSuffix  []byte
}

func (dw *DiffWriter) Write(ctx context.Context, dest *diffdoc.Hunk, rm1, rm2 record.Meta, pairs []record.Pair) {
	var err error
	var hunkHeader []byte

	defer func() {
		// We always seal the hunk. Note that hunkHeader is populated at the bottom
		// of the function. But if an error occurs and the function is returning
		// early, it's ok if hunkHeader is empty.
		dest.Seal(hunkHeader, err)
	}()

	if len(pairs) == 0 {
		return
	}

	buf1 := &bytes.Buffer{}
	csv1 := NewCommaRecordWriter(buf1, dw.pr)
	if err = csv1.Open(ctx, rm1); err != nil {
		dest.Seal(nil, err)
		return
	}
	buf2 := &bytes.Buffer{}
	csv2 := NewCommaRecordWriter(buf2, dw.pr)
	if err = csv2.Open(ctx, rm2); err != nil {
		dest.Seal(nil, err)
		return
	}

	recs := make([]record.Record, 1)
	var pair record.Pair
	var diffCount int

	for i := 0; i < len(pairs) && ctx.Err() == nil; i++ {
		pair = pairs[i]
		if pair.Equal() {
			recs[0] = pair.Rec1()
			_ = csv1.WriteRecords(ctx, recs)
			_ = csv1.Flush(ctx)
			_, _ = dest.Write(dw.contextPrefix)
			_, _ = dest.Write(buf1.Bytes()[0 : buf1.Len()-1])
			_, _ = dest.Write(dw.contextSuffix)
			buf1.Reset()
			continue
		}

		diffCount++
		recs[0] = pair.Rec1()
		_ = csv1.WriteRecords(ctx, recs)
		_ = csv1.Flush(ctx)
		_, _ = dest.Write(dw.deletePrefix)
		_, _ = dest.Write(buf1.Bytes()[0 : buf1.Len()-1])
		_, _ = dest.Write(dw.deleteSuffix)
		buf1.Reset()

		recs[0] = pair.Rec2()
		_ = csv2.WriteRecords(ctx, recs)
		_ = csv2.Flush(ctx)
		_, _ = dest.Write(dw.insertPrefix)
		_, _ = dest.Write(buf2.Bytes()[0 : buf2.Len()-1])
		_, _ = dest.Write(dw.insertSuffix)
		buf2.Reset()
	}

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	offset := dest.Offset() + 1
	var headerText string
	if len(pairs) == 1 {
		headerText = fmt.Sprintf("@@ -%d +%d @@", offset, offset)
	} else {
		headerText = fmt.Sprintf("@@ -%d,%d +%d,%d @@", offset, len(pairs), offset, len(pairs))
	}

	seq := colorz.ExtractSeqs(dw.pr.Diff.Section)
	hunkHeader = seq.Appendln(hunkHeader, []byte(headerText))
}
