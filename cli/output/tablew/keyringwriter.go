package tablew

import (
	"context"
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

var _ output.KeyringWriter = (*keyringWriter)(nil)

// keyringWriter is the text/table implementation of output.KeyringWriter.
type keyringWriter struct {
	tbl *table
	out io.Writer
	pr  *output.Printing
}

// NewKeyringWriter returns a text/table output.KeyringWriter.
func NewKeyringWriter(out io.Writer, pr *output.Printing) output.KeyringWriter {
	tbl := &table{out: out, pr: pr, header: pr.ShowHeader}
	tbl.reset()
	return &keyringWriter{tbl: tbl, out: out, pr: pr}
}

// List implements output.KeyringWriter.
func (w *keyringWriter) List(refs []output.KeyringRef) error {
	if len(refs) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(refs))
	for _, r := range refs {
		rows = append(rows, []string{r.Status, r.Path, r.Handle, r.Driver})
	}
	w.tbl.tblImpl.SetHeader([]string{"STATUS", "PATH", "HANDLE", "DRIVER"})
	w.tbl.tblImpl.SetColTrans(0, w.pr.Faint.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, w.pr.Faint.SprintFunc())
	return w.tbl.appendRowsAndRenderAll(context.TODO(), rows)
}

// Get implements output.KeyringWriter.
func (w *keyringWriter) Get(path, value string, revealed bool) error {
	var err error
	if revealed {
		_, err = fmt.Fprintln(w.out, value)
	} else {
		_, err = fmt.Fprintf(w.out, "secret exists: %s\n", w.pr.Handle.Sprint(path))
	}
	return errz.Err(err)
}

// Created implements output.KeyringWriter. Successful create is silent
// in text mode.
func (w *keyringWriter) Created(_ string) error {
	return nil
}

// Updated implements output.KeyringWriter. Successful update is silent
// in text mode.
func (w *keyringWriter) Updated(_ string) error {
	return nil
}

// Rm implements output.KeyringWriter. Successful rm is silent in text
// mode (matches the historical behavior of "sq config keyring rm").
func (w *keyringWriter) Rm(_ string) error {
	return nil
}

// Prune implements output.KeyringWriter. It renders an aligned table of the
// orphaned entries (PATH / KIND / STATUS). When there are none, it prints a
// short message instead of an empty table.
func (w *keyringWriter) Prune(rows []output.KeyringPruneRow, _ bool) error {
	if len(rows) == 0 {
		_, err := fmt.Fprint(w.out, w.pr.Faint.Sprint("No orphaned entries to prune.\n"))
		return errz.Err(err)
	}

	w.tbl.reset()
	tblRows := make([][]string, 0, len(rows))
	for _, r := range rows {
		// Pre-color per row: the action status is green, a failure is red,
		// and the KIND is muted as supplementary info.
		status := pruneStatusCell(r)
		if r.Status == output.KeyringPruneStatusFailed {
			status = w.pr.Error.Sprint(status)
		} else {
			status = w.pr.Enabled.Sprint(status)
		}
		tblRows = append(tblRows, []string{w.pr.String.Sprint(r.Path), w.pr.Faint.Sprint(r.Kind), status})
	}
	w.tbl.tblImpl.SetHeader([]string{"PATH", "KIND", "STATUS"})
	return w.tbl.appendRowsAndRenderAll(context.TODO(), tblRows)
}

// pruneStatusCell returns the STATUS cell for a prune row: "delete" for a
// dry-run plan, "deleted" for a removed entry, or "failed: <error>" for a
// deletion that failed.
func pruneStatusCell(r output.KeyringPruneRow) string {
	switch r.Status {
	case output.KeyringPruneStatusPlanned:
		return "delete"
	case output.KeyringPruneStatusDeleted:
		return "deleted"
	case output.KeyringPruneStatusFailed:
		if r.Error != "" {
			return "failed: " + r.Error
		}
		return "failed"
	default:
		return r.Status
	}
}

// Migrate implements output.KeyringWriter. It renders an aligned table.
// By default only actionable rows (migrate / migrated / failed) are shown;
// skipped sources are listed only when verbose output is enabled. When
// nothing is actionable, a short message is printed instead of an empty
// table.
func (w *keyringWriter) Migrate(rows []output.KeyringMigrateRow, _ bool) error {
	display := make([]output.KeyringMigrateRow, 0, len(rows))
	var skipped int
	for _, r := range rows {
		if r.Status == output.KeyringMigrateStatusSkip {
			skipped++
			if !w.pr.Verbose {
				continue
			}
		}
		display = append(display, r)
	}

	if len(display) == 0 {
		var msg string
		switch {
		case skipped == 1:
			msg = "Nothing to migrate (1 source skipped; use -v to see why).\n"
		case skipped > 1:
			msg = fmt.Sprintf("Nothing to migrate (%d sources skipped; use -v to see why).\n", skipped)
		default:
			msg = "Nothing to migrate.\n"
		}
		_, err := fmt.Fprint(w.out, w.pr.Faint.Sprint(msg))
		return errz.Err(err)
	}

	w.tbl.reset()
	tblRows := make([][]string, 0, len(display))
	for _, r := range display {
		handle, status, detail := w.colorMigrateRow(r)
		tblRows = append(tblRows, []string{handle, status, detail})
	}
	w.tbl.tblImpl.SetHeader([]string{"HANDLE", "STATUS", "DETAIL"})
	return w.tbl.appendRowsAndRenderAll(context.TODO(), tblRows)
}

// colorMigrateRow pre-colors a migrate row's cells. A skipped row is muted in
// full (it's benign and should recede behind the actionable rows); a
// migrate/migrated row gets a green status with a muted detail; a failed row
// gets a red status and error. Pre-coloring (rather than a column transform)
// is what lets the whole skip row be dimmed, and it stays aligned because the
// table measures column width with ANSI codes stripped.
func (w *keyringWriter) colorMigrateRow(r output.KeyringMigrateRow) (handle, status, detail string) {
	handle, status, detail = r.Handle, migrateStatusLabel(r.Status), migrateDetail(r)
	switch r.Status {
	case output.KeyringMigrateStatusSkip:
		return w.pr.Faint.Sprint(handle), w.pr.Faint.Sprint(status), w.pr.Faint.Sprint(detail)
	case output.KeyringMigrateStatusFailed:
		return w.pr.Handle.Sprint(handle), w.pr.Error.Sprint(status), w.pr.Error.Sprint(detail)
	default: // planned, migrated
		return w.pr.Handle.Sprint(handle), w.pr.Enabled.Sprint(status), w.pr.Faint.Sprint(detail)
	}
}

// migrateStatusLabel maps a KeyringMigrateStatus* value to its display word.
func migrateStatusLabel(status string) string {
	switch status {
	case output.KeyringMigrateStatusPlanned:
		return "migrate"
	case output.KeyringMigrateStatusMigrated:
		return "migrated"
	case output.KeyringMigrateStatusFailed:
		return "failed"
	case output.KeyringMigrateStatusSkip:
		return "skip"
	default:
		return status
	}
}

// migrateDetail returns the DETAIL cell for a migrate row: the target
// placeholder for a plan, the new location for a success, the error for a
// failure, or the skip reason.
func migrateDetail(r output.KeyringMigrateRow) string {
	switch r.Status {
	case output.KeyringMigrateStatusPlanned:
		return "${keyring:<new-id>}"
	case output.KeyringMigrateStatusMigrated:
		return r.NewLocation
	case output.KeyringMigrateStatusFailed:
		return r.Error
	case output.KeyringMigrateStatusSkip:
		return r.Reason
	default:
		return ""
	}
}
