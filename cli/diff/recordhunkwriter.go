package diff

import (
	"bytes"
	"context"
	"fmt"
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

	recs1 := make([]record.Record, 0)
	recs2 := make([]record.Record, 0)
	for i := range pairs {
		if rec := pairs[i].Rec1(); rec != nil {
			recs1 = append(recs1, rec)
		}
		if rec := pairs[i].Rec2(); rec != nil {
			recs2 = append(recs2, rec)
		}
	}

	g, gCtx := errgroup.WithContext(ctx)
	if len(recs1) > 0 {
		g.Go(func() error {
			var bodyErr error
			body1, bodyErr = wa.renderRecords(gCtx, rm1, recs1)
			return bodyErr
		})
	}

	if len(recs2) > 0 {
		g.Go(func() error {
			var bodyErr error
			body2, bodyErr = wa.renderRecords(gCtx, rm2, recs2)
			return bodyErr
		})
	}

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

	if err = diffdoc.ColorizeHunks(ctx, hunk, wa.pr.Diff, strings.NewReader(hunkBody)); err != nil {
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

// adjustHunkOffset adjusts the offset of a diff hunk. The hunk input is
// expected to be a string of one of two basic forms. This is the long form:
//
//	@@ -44,7 +44,7 @@
//
// Given an offset of 10, this would become:
//
//	@@ -54,7 +54,7 @@
//
// The short form is:
//
//	@@ -44 +44 @@
//
// Given an offset of 10, this would become:
//
//	@@ -54 +54 @@
//
// The short form is used when there's no surrounding lines (-U=0).
//
// Annoyingly, it's actually possible to find hybrid forms, such as:
//
//	@@ -44 +44,7 @@
//	@@ -44,7 +44 @@
//
// These hybrid forms are handled as expected.
//
// Note that "-44,7 +44,7" means that the first line shown is line 44 and the
// number of lines compared is 7 (although 8 lines will be rendered, because the
// changed line is shown twice: the before and after versions of the line).
func adjustHunkOffset(hunk string, offset int) (string, error) {
	// https://unix.stackexchange.com/questions/81998/understanding-of-diff-output
	const fm1 = "@@ -%d,%d +%d,%d @@"
	const fm2 = "@@ -%d +%d,%d @@"
	const fm3 = "@@ -%d,%d +%d @@"
	const fm4 = "@@ -%d +%d @@"

	var i1, i2, i3, i4 int
	count, err := fmt.Fscanf(strings.NewReader(hunk), fm1, &i1, &i2, &i3, &i4)
	if err == nil {
		if count != 4 {
			return "", errz.Errorf("expected 4 values, got %d", count)
		}

		i1 += offset
		i3 += offset

		return fmt.Sprintf(fm1, i1, i2, i3, i4), nil
	}

	count, err = fmt.Fscanf(strings.NewReader(hunk), fm2, &i1, &i3, &i4)
	if err == nil {
		if count != 3 {
			return "", errz.Errorf("expected 3 values, got %d", count)
		}

		i1 += offset
		i3 += offset

		return fmt.Sprintf(fm2, i1, i3, i4), nil
	}

	count, err = fmt.Fscanf(strings.NewReader(hunk), fm3, &i1, &i2, &i3)
	if err == nil {
		if count != 3 {
			return "", errz.Errorf("expected 3 values, got %d", count)
		}

		i1 += offset
		i3 += offset

		return fmt.Sprintf(fm3, i1, i2, i3), nil
	}

	// Long format didn't work, try the short format.
	_, err = fmt.Fscanf(strings.NewReader(hunk), fm4, &i1, &i3)
	if err != nil {
		return "", errz.Errorf("failed to parse Hunk: %s", hunk)
	}

	i1 += offset
	i3 += offset

	return fmt.Sprintf(fm4, i1, i3), nil
}
