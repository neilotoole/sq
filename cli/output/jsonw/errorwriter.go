package jsonw

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/cli/output"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	log *slog.Logger
	out io.Writer
	pr  *output.Printing
}

// NewErrorWriter returns an output.ErrorWriter that outputs in JSON.
func NewErrorWriter(log *slog.Logger, out io.Writer, pr *output.Printing) output.ErrorWriter {
	return &errorWriter{log: log, out: out, pr: pr}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(err error) {
	const tplNoPretty = "{%s: %s}"
	tplPretty := "{\n" + w.pr.Indent + "%s" + ": %s\n}"

	b, err2 := encodeString(nil, err.Error(), false)
	lg.WarnIfError(w.log, "encode JSON string", err2)

	key := w.pr.Key.Sprint(`"error"`)
	val := w.pr.Error.Sprint(string(b)) // trim the newline

	var s string
	if !w.pr.Compact {
		s = fmt.Sprintf(tplPretty, key, val)
	} else {
		s = fmt.Sprintf(tplNoPretty, key, val)
	}

	fmt.Fprintln(w.out, s)
}
