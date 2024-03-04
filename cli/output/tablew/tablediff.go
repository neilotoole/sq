package tablew

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/record"
)

func NewDiffWriter(pr *output.Printing) diff.RecordHunkWriter {
	pr = pr.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false

	dw := &diffWriter{
		pr: pr,
	}

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

// diffWriter is partially-optimized implementation of diff.RecordHunkWriter for
// format "text". It still delegates its record generation to this package's
// RecordWriter implementation, resulting in way too many allocations. It's
// actually pretty egregious in terms of resource usage.
type diffWriter struct {
	pr            *output.Printing
	contextPrefix []byte
	contextSuffix []byte
	insertPrefix  []byte
	insertSuffix  []byte
	deletePrefix  []byte
	deleteSuffix  []byte
}

// WriteHunk implements diff.RecordHunkWriter.
func (dw *diffWriter) WriteHunk(ctx context.Context, dest *diffdoc.Hunk, rm1, rm2 record.Meta, pairs []record.Pair) {
	if rm1.Equalish(rm2) {
		dw.writeEqualish(ctx, dest, rm1, rm2, pairs)
		return
	}

	dw.writeDifferent(ctx, dest, rm1, rm2, pairs)
}

//nolint:dupl
func (dw *diffWriter) writeDifferent(ctx context.Context, dest *diffdoc.Hunk,
	rm1, rm2 record.Meta, pairs []record.Pair,
) {
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

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	recs1 := make([]record.Record, 0)
	recs2 := make([]record.Record, 0)

	for i := 0; i < len(pairs) && ctx.Err() == nil; i++ {
		if rec := pairs[i].Rec1(); rec != nil {
			recs1 = append(recs1, rec)
		}
		if rec := pairs[i].Rec2(); rec != nil {
			recs2 = append(recs2, rec)
		}
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	buf1 := &bytes.Buffer{}
	tw := NewRecordWriter(buf1, dw.pr)
	if err = tw.Open(ctx, rm1); err != nil {
		return
	}
	if err = tw.WriteRecords(ctx, recs1); err != nil {
		return
	}
	if err = tw.Flush(ctx); err != nil {
		return
	}
	if err = tw.Close(ctx); err != nil {
		return
	}
	buf2 := &bytes.Buffer{}
	tw = NewRecordWriter(buf2, dw.pr)
	if err = tw.Open(ctx, rm2); err != nil {
		return
	}
	if err = tw.WriteRecords(ctx, recs2); err != nil {
		return
	}
	if err = tw.Flush(ctx); err != nil {
		return
	}
	if err = tw.Close(ctx); err != nil {
		return
	}

	sc1 := bufio.NewScanner(buf1)
	sc2 := bufio.NewScanner(buf2)
	var line []byte
	var i, j, k int
	for i = 0; i < len(pairs) && ctx.Err() == nil; i++ {
		if pairs[i].Equal() {
			_ = sc1.Scan()
			line = sc1.Bytes()
			_ = sc2.Scan()

			_, _ = dest.Write(dw.contextPrefix)
			_, _ = dest.Write(line[0 : len(line)-1]) // trim trailing newline
			_, _ = dest.Write(dw.contextSuffix)      // contains newline
			continue
		}

		// We've found a difference. We need to print all consecutive "deletion"
		// lines; and when those are done, we do the consecutive "insertion" lines.

		for j = i; ctx.Err() == nil && j < len(pairs) && !pairs[j].Equal() && sc1.Scan(); j++ {
			line = sc1.Bytes()
			_, _ = dest.Write(dw.deletePrefix)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.deleteSuffix)
		}
		if ctx.Err() != nil {
			err = context.Cause(ctx)
			return
		}

		for k = i; ctx.Err() == nil && k < len(pairs) && !pairs[k].Equal() && sc2.Scan(); k++ {
			line = sc2.Bytes()
			_, _ = dest.Write(dw.insertPrefix)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.insertSuffix)
		}
		if ctx.Err() != nil {
			err = context.Cause(ctx)
			return
		}

		// Adjust the main loop index to skip over the differing
		// records that we've just processed.
		i = max(j, k) - 1
	}

	offset := dest.Offset() + 1
	var headerText string
	if len(pairs) == 1 {
		// Short hunk header format for single-line diffs.
		headerText = fmt.Sprintf("@@ -%d +%d @@", offset, offset)
	} else {
		headerText = fmt.Sprintf("@@ -%d,%d +%d,%d @@", offset, len(pairs), offset, len(pairs))
	}

	seq := colorz.ExtractSeqs(dw.pr.Diff.Section)
	hunkHeader = seq.Appendln(hunkHeader, []byte(headerText))
}

// writeEqualish writes a hunk for records that are equal(ish). That is to say,
// the supplied record.Meta instances are effectively the same for our purposes.
//
//nolint:dupl
func (dw *diffWriter) writeEqualish(ctx context.Context, dest *diffdoc.Hunk,
	rm1, _ record.Meta, pairs []record.Pair,
) {
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

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	recs1 := make([]record.Record, 0)
	recs2 := make([]record.Record, 0)

	for i := 0; i < len(pairs) && ctx.Err() == nil; i++ {
		if rec := pairs[i].Rec1(); rec != nil {
			recs1 = append(recs1, rec)
		}
		if rec := pairs[i].Rec2(); rec != nil {
			recs2 = append(recs2, rec)
		}
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	count1 := len(recs1)
	count2 := len(recs2)
	allRecs := make([]record.Record, count1+count2)
	copy(allRecs, recs1)
	copy(allRecs[count1:], recs2)

	bufAllRecs := &bytes.Buffer{}
	tw := NewRecordWriter(bufAllRecs, dw.pr)
	if err = tw.Open(ctx, rm1); err != nil {
		return
	}
	if err = tw.WriteRecords(ctx, allRecs); err != nil {
		return
	}
	if err = tw.Flush(ctx); err != nil {
		return
	}
	if err = tw.Close(ctx); err != nil {
		return
	}

	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}

	sc := bufio.NewScanner(bufAllRecs)
	var i, j, k int
	for i = 0; ctx.Err() == nil && i < len(recs1) && sc.Scan(); i++ {
		_, _ = buf1.Write(sc.Bytes())
		buf1.WriteByte('\n')
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	for ; ctx.Err() == nil && i < len(allRecs) && sc.Scan(); i++ {
		_, _ = buf2.Write(sc.Bytes())
		buf2.WriteByte('\n')
	}
	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
	}

	buf1Str := buf1.String()
	buf2Str := buf2.String()
	_ = buf1Str
	_ = buf2Str

	sc1 := bufio.NewScanner(buf1)
	sc2 := bufio.NewScanner(buf2)
	var line []byte

	for i = 0; i < len(pairs) && ctx.Err() == nil; i++ {
		if pairs[i].Equal() {
			_ = sc1.Scan()
			line = sc1.Bytes()
			_ = sc2.Scan()

			_, _ = dest.Write(dw.contextPrefix)
			_, _ = dest.Write(line[0 : len(line)-1]) // trim trailing newline
			_, _ = dest.Write(dw.contextSuffix)      // contains newline
			continue
		}

		// We've found a difference. We need to print all consecutive "deletion"
		// lines; and when those are done, we do the consecutive "insertion" lines.

		for j = i; ctx.Err() == nil && j < len(pairs) && !pairs[j].Equal() && sc1.Scan(); j++ {
			line = sc1.Bytes()
			_, _ = dest.Write(dw.deletePrefix)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.deleteSuffix)
		}
		if ctx.Err() != nil {
			err = context.Cause(ctx)
			return
		}

		for k = i; ctx.Err() == nil && k < len(pairs) && !pairs[k].Equal() && sc2.Scan(); k++ {
			line = sc2.Bytes()
			_, _ = dest.Write(dw.insertPrefix)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.insertSuffix)
		}
		if ctx.Err() != nil {
			err = context.Cause(ctx)
			return
		}

		// Adjust the main loop index to skip over the differing
		// records that we've just processed.
		i = max(j, k) - 1
	}

	offset := dest.Offset() + 1
	var headerText string
	if len(pairs) == 1 {
		// Short hunk header format for single-line diffs.
		headerText = fmt.Sprintf("@@ -%d +%d @@", offset, offset)
	} else {
		headerText = fmt.Sprintf("@@ -%d,%d +%d,%d @@", offset, len(pairs), offset, len(pairs))
	}

	seq := colorz.ExtractSeqs(dw.pr.Diff.Section)
	hunkHeader = seq.Appendln(hunkHeader, []byte(headerText))
}
