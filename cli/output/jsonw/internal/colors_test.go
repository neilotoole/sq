package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
)

// TestNewColors_Clipped asserts each Color's Prefix and Suffix have no spare
// capacity. Both are cut from one buffer holding "prefix + space + suffix", so
// without clipping each retains the other's bytes and, worse, an append to one
// would write into the shared backing array rather than allocating. These
// slices outlive the buffer: they are handed to a long-lived *jsoncolor.Colors
// via JSONPalette, and jsoncolor.Color is an exported []byte that a consumer
// could append to.
func TestNewColors_Clipped(t *testing.T) {
	pr := output.NewPrinting()
	pr.EnableColor(true)
	require.False(t, pr.IsMonochrome())

	c := internal.NewColors(pr)

	cases := []struct {
		name string
		clr  internal.Color
	}{
		{"Null", c.Null},
		{"Bool", c.Bool},
		{"Number", c.Number},
		{"String", c.String},
		{"Key", c.Key},
		{"Bytes", c.Bytes},
		{"Time", c.Time},
		{"Punc", c.Punc},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEmpty(t, tc.clr.Prefix, "fixture should carry a prefix")
			require.Equal(t, len(tc.clr.Prefix), cap(tc.clr.Prefix),
				"Prefix must be clipped: len %d, cap %d",
				len(tc.clr.Prefix), cap(tc.clr.Prefix))
			require.Equal(t, len(tc.clr.Suffix), cap(tc.clr.Suffix),
				"Suffix must be clipped: len %d, cap %d",
				len(tc.clr.Suffix), cap(tc.clr.Suffix))
		})
	}
}
