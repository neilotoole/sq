package jsonw

import (
	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
)

// newJSONColorPalette returns the *jsoncolor.Colors for pr, or nil for a nil
// or monochrome Printing, which disables colorization at the encoder level.
//
// Both the mapping and the nil behavior live in internal.Colors.JSONPalette,
// so these writers and the internal package's own encode tests configure the
// encoder identically. See that method for why each Color's suffix is
// discarded.
func newJSONColorPalette(pr *output.Printing) *jsoncolor.Colors {
	return internal.NewColors(pr).JSONPalette()
}
