// Package diff contains the CLI's diff implementation.
//
// Reference:
// - https://github.com/aymanbagabas/go-udiff
// - https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
// - https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git
package diff

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	udiff "github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Config contains parameters to control diff behavior.
type Config struct {
	// RecordWriterFn is a factory function that returns
	// an output.RecordWriter used to generate diff text
	// when comparing table data.
	RecordWriterFn output.NewRecordWriterFunc
	// Lines specifies the number of lines of context surrounding a diff.
	Lines int

	pr *output.Printing
}

// Elements determines what source elements to compare.
type Elements struct {
	// Overview compares a summary of the sources.
	Overview bool

	// DBProperties compares DB properties.
	DBProperties bool

	// Schema compares table/schema structure.
	Schema bool

	// RowCount compares table row count when comparing schemata.
	RowCount bool

	// Data compares each row in a table. Caution: this can be slow.
	Data bool
}

// sourceData encapsulates data about a source.
type sourceData struct {
	src     *source.Source
	srcMeta *metadata.Source
	handle  string
}

func (sd *sourceData) clone() *sourceData { //nolint:unused // REVISIT: no longer needed?
	if sd == nil {
		return nil
	}

	return &sourceData{
		handle:  sd.handle,
		src:     sd.src.Clone(),
		srcMeta: sd.srcMeta.Clone(),
	}
}

// tableData encapsulates data about a table.
type tableData struct {
	tblMeta *metadata.Table
	src     *source.Source
	srcMeta *metadata.Source
	tblName string
}

// String returns @handle.table.
func (td *tableData) String() string {
	return fmt.Sprintf("%s.%s", td.src.Handle, td.tblName)
}

func (td *tableData) clone() *tableData { //nolint:unused // REVISIT: no longer needed?
	if td == nil {
		return nil
	}

	return &tableData{
		tblName: td.tblName,
		tblMeta: td.tblMeta.Clone(),
		src:     td.src.Clone(),
		srcMeta: td.srcMeta.Clone(),
	}
}

// sourceOverviewDiff is a container for a source overview diff.
type sourceOverviewDiff struct {
	sd1, sd2 *sourceData
	header   string
	diff     string
}

// tableDiff is a container for a table diff.
type tableDiff struct {
	td1, td2 *tableData
	header   string
	diff     string
}

// dbPropsDiff is a container for a DB properties diff.
type dbPropsDiff struct {
	sd1, sd2 *sourceData
	header   string
	diff     string
}

// tableDataDiff is a container for a table's data diff.
type tableDataDiff struct {
	td1, td2 *tableData
	// recMeta1, recMeta2 record.Meta
	header string
	diff   string
}

// Print prints dif to w. If pr is nil, printing is in monochrome.
func Print(ctx context.Context, w io.Writer, pr *output.Printing, header, dif string) error {
	if dif == "" {
		return nil
	}

	if pr == nil || pr.IsMonochrome() {
		if header != "" {
			dif = header + "\n" + dif
		}
		_, err := fmt.Fprintln(w, dif)
		return errz.Err(err)
	}

	bar := progress.FromContext(ctx).
		NewUnitCounter("Preparing diff output", "line", progress.OptMemUsage)

	// FIXME: Should stream diff line colorization, not buffer it all up.
	after := stringz.VisitLines(dif, func(i int, line string) string {
		if i == 0 && strings.HasPrefix(line, "---") {
			return pr.DiffHeader.Sprint(line)
		}
		if i == 1 && strings.HasPrefix(line, "+++") {
			return pr.DiffHeader.Sprint(line)
		}

		if strings.HasPrefix(line, "@@") {
			return pr.DiffSection.Sprint(line)
		}

		if strings.HasPrefix(line, "-") {
			return pr.DiffMinus.Sprint(line)
		}

		if strings.HasPrefix(line, "+") {
			return pr.DiffPlus.Sprint(line)
		}

		bar.Incr(1)
		return pr.DiffNormal.Sprint(line)
	})

	if header != "" {
		after = pr.DiffHeader.Sprint(header) + "\n" + after
	}

	bar.Stop()
	_, err := fmt.Fprintln(w, after)
	return errz.Err(err)
}

// Print2Colorize prints dif to w. If pr is nil, printing is in monochrome.
func Print2Colorize(ctx context.Context, w io.Writer, pr *output.Printing, header string, dif io.Reader) error {
	if dif == nil {
		return nil
	}

	var err error

	if pr == nil || pr.IsMonochrome() {
		// Monochrome
		if header != "" {
			if _, err = fmt.Fprintln(w, header); err != nil {
				return errz.Err(err)
			}
		}
		_, err = fmt.Fprintln(w, dif)
		return errz.Err(err)
	}

	// Colorized.

	if header != "" {
		if _, err = pr.DiffHeader.Fprintln(w, header); err != nil {
			return errz.Err(err)
		}
	}

	sc := bufio.NewScanner(dif)
	for i := 0; sc.Scan(); i++ {
		line := sc.Text()
		// REVISIT: ^^ don't need to allocate here.
		// Instead, use sc.Bytes()

		switch {
		case i == 0 && strings.HasPrefix(line, "---"):
			_, err = pr.DiffHeader.Fprintln(w, line)
		case i == 1 && strings.HasPrefix(line, "+++"):
			_, err = pr.DiffHeader.Fprintln(w, line)
		case strings.HasPrefix(line, "@@"):
			_, err = pr.DiffSection.Fprintln(w, line)
		case strings.HasPrefix(line, "-"):
			_, err = pr.DiffMinus.Fprintln(w, line)
		case strings.HasPrefix(line, "+"):
			_, err = pr.DiffPlus.Fprintln(w, line)
		default:
			_, err = pr.DiffNormal.Fprintln(w, line)
		}

		if err != nil {
			return errz.Err(err)
		}
	}

	return errz.Err(sc.Err())

	//if i == 0 && strings.HasPrefix(line, "---") {
	//	if _, err = pr.DiffHeader.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//} else if i == 1 && strings.HasPrefix(line, "+++") {
	//	if _, err = pr.DiffHeader.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//} else if strings.HasPrefix(line, "@@") {
	//	if _, err = pr.DiffSection.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//} else if strings.HasPrefix(line, "-") {
	//	if _, err = pr.DiffMinus.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//} else if strings.HasPrefix(line, "+") {
	//	if _, err = pr.DiffPlus.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//} else {
	//	if _, err = pr.DiffNormal.Line(w, line); err != nil {
	//		return errz.Err(err)
	//	}
	//}

	//// FIXME: Should stream diff line colorization, not buffer it all up.
	//after := stringz.VisitLines(dif, func(i int, line string) string {
	//	if i == 0 && strings.HasPrefix(line, "---") {
	//		return pr.DiffHeader.Sprint(line)
	//	}
	//	if i == 1 && strings.HasPrefix(line, "+++") {
	//		return pr.DiffHeader.Sprint(line)
	//	}
	//
	//	if strings.HasPrefix(line, "@@") {
	//		return pr.DiffSection.Sprint(line)
	//	}
	//
	//	if strings.HasPrefix(line, "-") {
	//		return pr.DiffMinus.Sprint(line)
	//	}
	//
	//	if strings.HasPrefix(line, "+") {
	//		return pr.DiffPlus.Sprint(line)
	//	}
	//
	//	return pr.DiffNormal.Sprint(line)
	//})
	//
	//if header != "" {
	//	after = pr.DiffHeader.Sprint(header) + "\n" + after
	//}
	//
	//bar.Stop()
	//_, err := fmt.Line(w, after)
	//return errz.Err(err)
}

// computeUnified encapsulates computing a unified diff.
func computeUnified(ctx context.Context, msg, oldLabel, newLabel string, lines int,
	before, after string,
) (string, error) {
	if msg == "" {
		msg = "Diffing"
	} else {
		msg = fmt.Sprintf("Diffing (%s)", msg)
	}

	bar := progress.FromContext(ctx).NewWaiter(msg, true, progress.OptMemUsage)
	defer bar.Stop()

	var (
		unified string
		err     error
		done    = make(chan struct{})
	)

	// We compute the diff on a goroutine because the underlying diff
	// library functions aren't context-aware.
	go func() {
		defer close(done)

		edits := myers.ComputeEdits(before, after)
		// After edits are computed, if the context is done,
		// there's no point continuing.
		select {
		case <-ctx.Done():
			err = errz.Err(ctx.Err())
			return
		default:
		}

		unified, err = udiff.ToUnified(
			oldLabel,
			newLabel,
			before,
			edits,
			lines,
		)
	}()

	select {
	case <-ctx.Done():
		return "", errz.Err(ctx.Err())
	case <-done:
	}

	return unified, err
}
