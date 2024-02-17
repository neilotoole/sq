// Package diff contains sq's diff implementation.
package diff

import (
	"fmt"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Config contains parameters to control diff behavior.
type Config struct {
	// RecordWriterFn is a factory function that returns
	// an output.RecordWriter used to generate diff text
	// when comparing table data.
	RecordWriterFn output.NewRecordWriterFunc

	prMain *output.Printing
	prDiff *libdiff.Colors

	// Lines specifies the number of lines of context surrounding a diff.
	Lines int

	// HunkMaxSize specifies the maximum number of items in a diff hunk.
	HunkMaxSize int
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
