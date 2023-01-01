package jsonw

import (
	"fmt"
	"io"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/cli/output"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	log lg.Log
	out io.Writer
	fm  *output.Formatting
}

// NewErrorWriter returns an output.ErrorWriter that outputs in JSON.
func NewErrorWriter(log lg.Log, out io.Writer, fm *output.Formatting) output.ErrorWriter {
	return &errorWriter{log: log, out: out, fm: fm}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(err error) {
	const tplNoPretty = "{%s: %s}"
	tplPretty := "{\n" + w.fm.Indent + "%s" + ": %s\n}"

	b, err2 := encodeString(nil, err.Error(), false)
	w.log.WarnIfError(err2)

	key := w.fm.Key.Sprint(`"error"`)
	val := w.fm.Error.Sprint(string(b)) // trim the newline

	var s string
	if w.fm.Pretty {
		s = fmt.Sprintf(tplPretty, key, val)
	} else {
		s = fmt.Sprintf(tplNoPretty, key, val)
	}

	fmt.Fprintln(w.out, s)
}
