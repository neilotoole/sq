// Package jsonw implements output writers for JSON.
package jsonw

import (
	"io"

	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// WriteJSON prints a JSON representation of v to out, using specs
// from pr. It honors pr.Compact, pr.Indent, and the color palette
// derived from pr. The underlying [jsoncolor.Encoder.Encode] always
// appends a trailing newline, so callers do not (and must not) write
// one themselves.
func WriteJSON(out io.Writer, pr *output.Printing, v any) error {
	return writeJSON(out, pr, v)
}

// writeJSON prints a JSON representation of v to out, using specs
// from pr.
func writeJSON(out io.Writer, pr *output.Printing, v any) error {
	enc := jsoncolor.NewEncoder(out)
	enc.SetColors(newJSONColorPalette(pr))
	enc.SetEscapeHTML(false)
	if !pr.Compact {
		enc.SetIndent("", pr.Indent)
	}

	if err := enc.Encode(v); err != nil {
		return errz.Err(err)
	}
	return nil
}
