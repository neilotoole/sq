package internal

import (
	"github.com/neilotoole/jsoncolor"
)

// JSONPalette returns c as a *jsoncolor.Colors, or nil if c carries no
// prefixes at all. The encoder treats a nil palette as "no colorization",
// which is what NewColors yields for a nil or monochrome Printing.
//
// A jsoncolor.Color is the ANSI prefix only: jsoncolor emits its own fixed
// reset after every colored token, whereas fatih/color emits
// attribute-specific resets (\x1b[22m for Bold/Faint, for instance). The two
// are visually equivalent but not byte-identical, so each Color's Suffix is
// deliberately discarded in favor of jsoncolor's reset.
//
// This is the only conversion from Colors to a jsoncolor palette. Both the
// JSON writers and this package's encode tests go through it, so the encoder
// is configured identically in production and under test.
func (c Colors) JSONPalette() *jsoncolor.Colors {
	if len(c.Null.Prefix) == 0 && len(c.Bool.Prefix) == 0 &&
		len(c.Number.Prefix) == 0 && len(c.String.Prefix) == 0 &&
		len(c.Key.Prefix) == 0 && len(c.Bytes.Prefix) == 0 &&
		len(c.Time.Prefix) == 0 && len(c.Punc.Prefix) == 0 {
		return nil
	}

	return &jsoncolor.Colors{
		Null:   c.Null.Prefix,
		Bool:   c.Bool.Prefix,
		Number: c.Number.Prefix,
		String: c.String.Prefix,
		Key:    c.Key.Prefix,
		Bytes:  c.Bytes.Prefix,
		Time:   c.Time.Prefix,
		Punc:   c.Punc.Prefix,
	}
}
