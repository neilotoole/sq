package csvw

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/neilotoole/sq/cli/diff"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
)

// NewCommaDiffWriter returns a diff.RecordHunkWriter for CSV.
func NewCommaDiffWriter(pr *output.Printing) diff.RecordHunkWriter {
	return newDiffWriter(pr, NewCommaRecordWriter)
}

// NewTabDiffWriter returns a diff.RecordHunkWriter for TSV.
func NewTabDiffWriter(pr *output.Printing) diff.RecordHunkWriter {
	return newDiffWriter(pr, NewTabRecordWriter)
}

func newDiffWriter(pr *output.Printing, rw output.NewRecordWriterFunc) diff.RecordHunkWriter {
	pr = pr.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false

	dw := &diffWriter{
		pr:          pr,
		newWriterFn: rw,
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

// diffWriter is an implementation of diff.RecordHunkWriter for CSV/TSV. It
// still delegates its CSV record generation to this package's RecordWriter
// implementation, which itself delegates to stdlib encoding/csv, resulting in
// way too many allocations. The entire thing needs to be reimplemented.
type diffWriter struct {
	pr            *output.Printing
	newWriterFn   output.NewRecordWriterFunc
	contextPrefix []byte
	contextSuffix []byte
	insertPrefix  []byte
	insertSuffix  []byte
	deletePrefix  []byte
	deleteSuffix  []byte
}

// WriteHunk implements diff.RecordHunkWriter.
func (dw *diffWriter) WriteHunk(ctx context.Context, dest *diffdoc.Hunk, rm1, rm2 record.Meta, pairs []record.Pair) {
	log := lg.FromContext(ctx)
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

	buf1 := dw.pr.NewBufferFn()
	defer lg.WarnIfCloseError(log, lgm.CloseBuffer, buf1)
	csv1 := dw.newWriterFn(buf1, dw.pr)
	if err = csv1.Open(ctx, rm1); err != nil {
		dest.Seal(nil, err)
		return
	}
	buf2 := dw.pr.NewBufferFn()
	defer lg.WarnIfCloseError(log, lgm.CloseBuffer, buf2)
	csv2 := dw.newWriterFn(buf2, dw.pr)
	if err = csv2.Open(ctx, rm2); err != nil {
		dest.Seal(nil, err)
		return
	}

	// recs is a slice of length 1, which we reuse for writing records.
	recs := make([]record.Record, 1)
	var line []byte

	var i int
	for i = 0; i < len(pairs) && ctx.Err() == nil; i++ {
		if pairs[i].Equal() {
			// The record pair is equal, so we just need to print the record once.
			recs[0] = pairs[i].Rec1()
			_ = csv1.WriteRecords(ctx, recs)
			_ = csv1.Flush(ctx)
			_, _ = dest.Write(dw.contextPrefix)
			line, _ = io.ReadAll(buf1)
			_, _ = dest.Write(line[0 : len(line)-1]) // trim trailing newline
			_, _ = dest.Write(dw.contextSuffix)      // contains newline
			buf1.Reset()
			continue
		}

		// We've found a difference: a contiguous run of differing pairs. We print
		// all the run's "deletion" lines first, then all its "insertion" lines. A
		// pair may be single-sided (added: Rec1()==nil; removed: Rec2()==nil) or
		// two-sided (changed). We make TWO FULL passes over the run, SKIPPING (not
		// breaking on) the nil side, so a changed pair adjacent to an added/removed
		// pair still gets both its lines rendered (issue #947).

		// Find the end of this contiguous run of differing pairs.
		end := i
		for end < len(pairs) && !pairs[end].Equal() {
			end++
		}

		// Deletion pass: every pair with a left side (removed + changed).
		//
		// -38,TOM,MCKELLEN,2006-02-15T04:34:33Z
		// -39,GOLDIE,BRODY,2006-02-15T04:34:33Z
		for p := i; ctx.Err() == nil && p < end; p++ {
			recs[0] = pairs[p].Rec1()
			if recs[0] == nil {
				continue
			}
			_ = csv1.WriteRecords(ctx, recs)
			_ = csv1.Flush(ctx)
			_, _ = dest.Write(dw.deletePrefix)
			line, _ = io.ReadAll(buf1)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.deleteSuffix)
			buf1.Reset()
		}

		// Insertion pass: every pair with a right side (added + changed).
		//
		// +38,THOMAS,MCKELLEN,2006-02-15T04:34:33Z
		// +39,GOLDIE,LOCKS,2006-02-15T04:34:33Z
		for p := i; ctx.Err() == nil && p < end; p++ {
			recs[0] = pairs[p].Rec2()
			if recs[0] == nil {
				continue
			}
			_ = csv2.WriteRecords(ctx, recs)
			_ = csv2.Flush(ctx)
			_, _ = dest.Write(dw.insertPrefix)
			line, _ = io.ReadAll(buf2)
			_, _ = dest.Write(line[0 : len(line)-1])
			_, _ = dest.Write(dw.insertSuffix)
			buf2.Reset()
		}

		// Adjust the main loop index to skip over the differing
		// records that we've just processed.
		i = end - 1
	}

	if ctx.Err() != nil {
		err = context.Cause(ctx)
		return
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
