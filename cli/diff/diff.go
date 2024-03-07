// Package diff contains sq's diff implementation. There are two package
// entrypoints: ExecSourceDiff and ExecTableDiff.
package diff

import (
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/ioz"
)

// Config contains parameters to control diff behavior.
type Config struct {
	// Run is the main program run.Run instance.
	Run *run.Run

	// Modes specifies what diff modes to use.
	Modes *Modes

	// RecordHunkWriter generates a diff hunk for pairs of records.
	RecordHunkWriter RecordHunkWriter

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

	// StopAfter specifies the number of diffs to execute before stopping.
	StopAfter int
}

// Modes determines what diff modes to execute.
type Modes struct {
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

// getBufferFactor returns a diffdoc.Opt for use with [diffdoc.NewUnifiedDoc]
// or [diffdoc.NewHunkDoc] that configures the [diffdoc.Doc] to use buffers
// created by cfg.Run.Files. These buffers spill over to disk after a size
// threshold, which is helpful when diffing large files.
func getBufFactory(cfg *Config) diffdoc.Opt {
	if cfg == nil || cfg.Run == nil || cfg.Run.Files == nil {
		return diffdoc.OptBufferFactory(ioz.NewDefaultBuffer)
	}

	return diffdoc.OptBufferFactory(cfg.Run.Files.NewBuffer)
}
