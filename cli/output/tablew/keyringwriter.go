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

// Prune implements output.KeyringWriter.
func (w *keyringWriter) Prune(rows []output.KeyringPruneRow, _ bool) error {
	for _, r := range rows {
		path := w.pr.String.Sprint(r.Path)
		kind := w.pr.Faint.Sprint("(" + r.Kind + ")")
		var line string
		switch r.Status {
		case output.KeyringPruneStatusPlanned:
			line = fmt.Sprintf("%s  %s  %s\n", w.pr.Faint.Sprint("would delete"), path, kind)
		case output.KeyringPruneStatusDeleted:
			line = fmt.Sprintf("%s  %s  %s\n", w.pr.Enabled.Sprint("deleted"), path, kind)
		case output.KeyringPruneStatusFailed:
			line = fmt.Sprintf("%s  %s  %s  %s\n", w.pr.Error.Sprint("FAIL"), path, kind, r.Error)
		default:
			line = path + "  " + r.Status + "\n"
		}
		if _, err := fmt.Fprint(w.out, line); err != nil {
			return errz.Err(err)
		}
	}
	return nil
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
		tblRows = append(tblRows, []string{r.Handle, migrateStatusLabel(r.Status), migrateDetail(r)})
	}
	w.tbl.tblImpl.SetHeader([]string{"HANDLE", "STATUS", "DETAIL"})
	w.tbl.tblImpl.SetColTrans(0, w.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.migrateStatusTrans)
	w.tbl.tblImpl.SetColTrans(2, w.pr.Faint.SprintFunc())
	return w.tbl.appendRowsAndRenderAll(context.TODO(), tblRows)
}

// migrateStatusTrans colors a migrate STATUS cell by its value. Its
// signature matches the table writer's textTransFunc (func(...any) string).
func (w *keyringWriter) migrateStatusTrans(a ...any) string {
	s := fmt.Sprint(a...)
	switch s {
	case "migrate", "migrated":
		return w.pr.Enabled.Sprint(s)
	case "failed":
		return w.pr.Error.Sprint(s)
	case "skip":
		return w.pr.Faint.Sprint(s)
	default:
		return s
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
