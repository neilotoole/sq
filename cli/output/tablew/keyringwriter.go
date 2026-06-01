package tablew

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

var _ output.KeyringWriter = (*keyringWriter)(nil)

// keyringWriter is the text/table implementation of output.KeyringWriter.
// It reproduces the human-readable output that the keyring subcommands
// used before the writer abstraction was introduced.
type keyringWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewKeyringWriter returns a text/table output.KeyringWriter.
func NewKeyringWriter(out io.Writer, pr *output.Printing) output.KeyringWriter {
	return &keyringWriter{out: out, pr: pr}
}

// List implements output.KeyringWriter.
func (w *keyringWriter) List(refs []output.KeyringRef) error {
	if len(refs) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)
	for _, r := range refs {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Path, r.Handle, r.Driver); err != nil {
			return errz.Err(err)
		}
	}
	return errz.Err(tw.Flush())
}

// Get implements output.KeyringWriter.
func (w *keyringWriter) Get(path, value string, revealed bool) error {
	var err error
	if revealed {
		_, err = fmt.Fprintln(w.out, value)
	} else {
		_, err = fmt.Fprintf(w.out, "secret exists: %s\n", path)
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
		var line string
		switch r.Status {
		case output.KeyringMigrateStatusSkip:
			line = fmt.Sprintf("%s  skip   (%s)\n", r.Handle, r.Reason)
		case output.KeyringMigrateStatusPlanned:
			line = r.Handle + "  ->     ${keyring:<new-id>}\n"
		case output.KeyringMigrateStatusMigrated:
			line = fmt.Sprintf("%s  done   ->  %s\n", r.Handle, r.NewLocation)
		case output.KeyringMigrateStatusFailed:
			line = fmt.Sprintf("%s  FAIL   %s\n", r.Handle, r.Error)
		default:
			line = r.Handle + "  " + r.Status + "\n"
		}
		if _, err := fmt.Fprint(w.out, line); err != nil {
			return errz.Err(err)
		}
	}
	return nil
}
