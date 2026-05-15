// Package jsonw implements output writers for JSON.
package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	"github.com/neilotoole/sq/cli/output/jsonw/internal/jcolorenc"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// WriteJSON prints a JSON representation of v to out, using specs
// from pr. It honours pr.Compact, pr.Indent, and the colour palette
// derived from pr.
func WriteJSON(out io.Writer, pr *output.Printing, v any) error {
	return writeJSON(out, pr, v)
}

// writeJSON prints a JSON representation of v to out, using specs
// from pr.
func writeJSON(out io.Writer, pr *output.Printing, v any) error {
	enc := jcolorenc.NewEncoder(out)
	enc.SetColors(internal.NewColors(pr))
	enc.SetEscapeHTML(false)
	if !pr.Compact {
		enc.SetIndent("", pr.Indent)
	}

	err := enc.Encode(v)
	if err != nil {
		return errz.Err(err)
	}

	return nil
}
