// Package diff contains sq's diff implementation. There are two package
// entrypoints: ExecSourceDiff and ExecTableDiff.
package diff

import (
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Config contains parameters to control diff behavior.
type Config struct {
	// Run is the main program run.Run instance.
	Run *run.Run

	// Elements specifies what elements to diff.
	Elements *Elements

	// RecordWriterFn is a factory function that returns
	// an output.RecordWriter used to generate diff text
	// when comparing table data.
	RecordWriterFn output.NewRecordWriterFunc

	// Printing is the output.Printing instance to use when generating diff text.
	Printing *output.Printing

	// Colors is the diff colors to use when generating diff text. It may be
	// modified by the diff package; pass a clone if the original should not be
	// modified.
	Colors *diffdoc.Colors

	// Lines specifies the number of lines of context surrounding a diff.
	Lines int

	// HunkMaxSize specifies the maximum number of items in a diff hunk.
	HunkMaxSize int

	// Concurrency specifies the maximum number of concurrent diff executions.
	// Zero indicates sequential execution; a negative values indicates unbounded
	// concurrency.
	Concurrency int

	// cache is lazily initialized by Config.init.
	cache *cache
}

// init lazy-initializes Config. It must be invoked at package entrypoints.
func (c *Config) init() {
	c.cache = &cache{
		ru:      c.Run,
		tblMeta: map[source.Table]*metadata.Table{},
	}
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
