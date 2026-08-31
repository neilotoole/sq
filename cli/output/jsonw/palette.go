package jsonw

import (
	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
)

// newJSONColorPalette returns a *jsoncolor.Colors built from pr's
// fatih/color fields. Returns nil for nil or monochrome Printing,
// which disables colorization at the encoder level.
//
// jsoncolor.Color is just a []byte of the ANSI prefix; jsoncolor emits
// a hardcoded \x1b[0m as the suffix after every colored token.
// fatih/color emits attribute-specific resets (e.g. \x1b[22m for
// Bold/Faint), so the two reset sequences are visually equivalent but
// not byte-identical. The adapter intentionally discards fatih/color's
// suffix in favor of jsoncolor's own reset.
func newJSONColorPalette(pr *output.Printing) *jsoncolor.Colors {
	if pr == nil || pr.IsMonochrome() {
		return nil
	}

	colors := internal.NewColors(pr)
	return &jsoncolor.Colors{
		Null:   colors.Null.Prefix,
		Bool:   colors.Bool.Prefix,
		Number: colors.Number.Prefix,
		String: colors.String.Prefix,
		Key:    colors.Key.Prefix,
		Bytes:  colors.Bytes.Prefix,
		Time:   colors.Time.Prefix,
		Punc:   colors.Punc.Prefix,
	}
}
