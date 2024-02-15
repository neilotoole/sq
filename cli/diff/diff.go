// Package diff contains sq's diff implementation.
//
// Reference:
// - https://github.com/aymanbagabas/go-udiff
// - https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
// - https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git
package diff

import (
	"context"
	"fmt"
	udiff "github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/progress"
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
