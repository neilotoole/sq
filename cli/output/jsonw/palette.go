package jsonw

import (
	"bytes"

	"github.com/fatih/color"

	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/cli/output"
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

	return &jsoncolor.Colors{
		Null:   jsonColorPrefix(pr.Null),
		Bool:   jsonColorPrefix(pr.Bool),
		Number: jsonColorPrefix(pr.Number),
		String: jsonColorPrefix(pr.String),
		Key:    jsonColorPrefix(pr.Key),
		Bytes:  jsonColorPrefix(pr.Bytes),
		Time:   jsonColorPrefix(pr.Datetime),
		Punc:   jsonColorPrefix(pr.Punc),
	}
}

// jsonColorPrefix returns the ANSI prefix bytes that c writes before a
// value. It mirrors the prefix-extraction trick from internal.newColor
// but discards the suffix (jsoncolor uses a fixed reset).
func jsonColorPrefix(c *color.Color) jsoncolor.Color {
	if c == nil {
		return nil
	}
	c2 := *c
	c2.EnableColor()
	b := []byte(c2.Sprint(" "))
	i := bytes.IndexByte(b, ' ')
	if i <= 0 {
		return nil
	}
	return jsoncolor.Color(b[:i])
}
