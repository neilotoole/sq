package diff

import (
	"bytes"
	"context"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// RecordHunkWriter generates a diff hunk for pairs of records in a particular
// diff output format.
type RecordHunkWriter interface {
	WriteHunk(ctx context.Context, dest *diffdoc.Hunk, rm1, rm2 record.Meta, pairs []record.Pair)
}

// NewRecordHunkWriterAdapter bridges RecordHunkWriter and output.RecordWriter
// for diff output formats that don't implement a native RecordHunkWriter.
func NewRecordHunkWriterAdapter(pr *output.Printing, rw output.NewRecordWriterFunc, lines int) RecordHunkWriter {
	return &recordHunkWriterAdapter{pr: pr, recWriterFn: rw, lines: lines}
}

type recordHunkWriterAdapter struct {
	pr          *output.Printing
	recWriterFn output.NewRecordWriterFunc
	// lines specifies the number of lines of context surrounding a diff.
	lines int
}

// WriteHunk implements RecordHunkWriter.
func (wa *recordHunkWriterAdapter) WriteHunk(ctx context.Context, hunk *diffdoc.Hunk,
	rm1, rm2 record.Meta, pairs []record.Pair,
) {
	var (
		hunkHeader, hunkBody string
		body1, body2         []byte
		err                  error
	)

	defer func() {
		// We always seal the hunk. Note that hunkHeader is populated at the bottom
		// of the function. But if an error occurs and the function is returning
		// early, it's ok if hunkHeader is empty.
		hunk.Seal([]byte(hunkHeader), err)
	}()

	recs1 := make([]record.Record, len(pairs))
	recs2 := make([]record.Record, len(pairs))
	for i := range pairs {
		recs1[i] = pairs[i].Rec1()
		recs2[i] = pairs[i].Rec2()
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var bodyErr error
		body1, bodyErr = wa.renderRecords(gCtx, rm1, recs1)
		return bodyErr
	})
	g.Go(func() error {
		var bodyErr error
		body2, bodyErr = wa.renderRecords(gCtx, rm2, recs2)
		return bodyErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = diffdoc.ComputeUnified(
		ctx,
		"recs1",
		"recs2",
		wa.lines,
		stringz.UnsafeString(body1),
		stringz.UnsafeString(body2),
	); err != nil {
		return
	}

	body1 = nil
	body2 = nil

	if unified == "" {
		// No diff was found.
		return
	}

	// Trim the diff "file header"... ultimately, we should change ComputeUnified
	// to not return this (e.g. add an arg "noHeader=true")
	trimmed := stringz.TrimHead(unified, 2)

	var ok bool
	if hunkHeader, hunkBody, ok = strings.Cut(trimmed, "\n"); !ok {
		err = errz.New("hunk header not found")
		return
	}

	if err = diffdoc.ColorizeHunks(ctx, hunk, wa.pr.Diff, bytes.NewReader(stringz.UnsafeBytes(hunkBody))); err != nil {
		return
	}

	if hunkHeader, err = adjustHunkOffset(hunkHeader, hunk.Offset()); err != nil {
		return
	}

	// hunkHeader will be passed to hunk.Seal in the top defer.
	hunkHeader = wa.pr.Diff.Section.Sprintln(hunkHeader)
}

func (wa *recordHunkWriterAdapter) renderRecords(ctx context.Context,
	recMeta record.Meta, recs []record.Record,
) ([]byte, error) {
	if len(recs) == 0 {
		return nil, nil
	}

	pr := wa.pr.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false
	buf := &bytes.Buffer{}
	recw := wa.recWriterFn(buf, pr)

	var err error
	if err = recw.Open(ctx, recMeta); err != nil {
		return nil, err
	}
	if err = recw.WriteRecords(ctx, recs); err != nil {
		return nil, err
	}
	if err = recw.Flush(ctx); err != nil {
		return nil, err
	}
	if err = recw.Close(ctx); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
