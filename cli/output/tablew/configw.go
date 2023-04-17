package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, fm *output.Formatting) output.ConfigWriter {
	return &configWriter{out: out, fm: fm}
}

// Dir implements output.ConfigWriter.
func (w *configWriter) Location(path, origin string) error {
	fmt.Fprintln(w.out, path)
	if w.fm.Verbose && origin != "" {
		w.fm.Faint.Fprint(w.out, "Origin: ")
		w.fm.String.Fprintln(w.out, origin)
	}

	return nil
}
