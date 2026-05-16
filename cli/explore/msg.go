// Package explore implements the `sq explore` TUI metadata explorer.
package explore

import (
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// quitMsg signals that the TUI should exit cleanly. It is dispatched
// by the quit key binding and may also carry an "emit handle" payload
// the wrapping command writes to stdout after [tea.Program.Run] returns.
//
//nolint:unused // dispatched in later phases.
type quitMsg struct {
	emitHandle string // empty if --emit-handle was not set
}

// sourceOverviewLoadedMsg is dispatched when a source's overview
// metadata (noSchema=true) returns. err is non-nil if the load failed;
// the UI surfaces the error inline rather than exiting.
type sourceOverviewLoadedMsg struct {
	meta   *metadata.Source
	err    error
	handle string
}

// tableNamesLoadedMsg is dispatched when MDCache.TableNames returns
// for handle.
type tableNamesLoadedMsg struct {
	err    error
	handle string
	names  []string
}

// tableMetaLoadedMsg is dispatched when MDCache.TableMeta returns
// for handle.tableName.
type tableMetaLoadedMsg struct {
	meta      *metadata.Table
	err       error
	handle    string
	tableName string
}

// previewMetaLoadedMsg is dispatched by the preview RecordWriter
// when the record-stream's header (column meta) arrives.
type previewMetaLoadedMsg struct {
	handle    string
	tableName string
	recMeta   record.Meta
}

// previewRowsAppendedMsg carries a batch of preview rows from the
// custom RecordWriter to the detail pane.
type previewRowsAppendedMsg struct {
	handle    string
	tableName string
	rows      []record.Record
	done      bool // true when the upstream stream has finished
}

// previewErrMsg is dispatched when the preview pipeline errors.
//
//nolint:unused // dispatched in later phases.
type previewErrMsg struct {
	err       error
	handle    string
	tableName string
}
