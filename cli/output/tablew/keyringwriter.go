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
		rows = append(rows, []string{r.Path, r.Handle, r.Driver})
	}
	w.tbl.tblImpl.SetHeader([]string{"PATH", "HANDLE", "DRIVER"})
	w.tbl.tblImpl.SetColTrans(0, w.pr.String.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, w.pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, w.pr.Faint.SprintFunc())
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

// Set implements output.KeyringWriter. Successful set is silent in text
// mode (matches the historical behavior of "sq config keyring set").
func (w *keyringWriter) Set(_ string) error {
	return nil
}

// Rm implements output.KeyringWriter. Successful rm is silent in text
// mode (matches the historical behavior of "sq config keyring rm").
func (w *keyringWriter) Rm(_ string) error {
	return nil
}

// Migrate implements output.KeyringWriter.
func (w *keyringWriter) Migrate(rows []output.KeyringMigrateRow, _ bool) error {
	for _, r := range rows {
		handle := w.pr.Handle.Sprint(r.Handle)
		var line string
		switch r.Status {
		case output.KeyringMigrateStatusSkip:
			line = fmt.Sprintf("%s  %s   (%s)\n", handle, w.pr.Faint.Sprint("skip"), r.Reason)
		case output.KeyringMigrateStatusPlanned:
			line = fmt.Sprintf("%s  %s     %s\n", handle, w.pr.Faint.Sprint("->"), "${keyring:<new-id>}")
		case output.KeyringMigrateStatusMigrated:
			line = fmt.Sprintf("%s  %s   ->  %s\n", handle, w.pr.Enabled.Sprint("done"), r.NewLocation)
		case output.KeyringMigrateStatusFailed:
			line = fmt.Sprintf("%s  %s   %s\n", handle, w.pr.Error.Sprint("FAIL"), r.Error)
		default:
			line = handle + "  " + r.Status + "\n"
		}
		if _, err := fmt.Fprint(w.out, line); err != nil {
			return errz.Err(err)
		}
	}
	return nil
}
