package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
)

// TestColors_JSONPalette covers the single conversion from internal.Colors to
// a *jsoncolor.Colors. Both the JSON writers and this package's own encode
// tests go through it, so the encoder is configured identically in production
// and under test.
func TestColors_JSONPalette(t *testing.T) {
	t.Run("zero_value_is_nil", func(t *testing.T) {
		require.Nil(t, internal.Colors{}.JSONPalette(),
			"a Colors carrying no prefixes means no colorization")
	})

	t.Run("nil_printing_is_nil", func(t *testing.T) {
		require.Nil(t, internal.NewColors(nil).JSONPalette())
	})

	t.Run("monochrome_is_nil", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(false)
		require.True(t, pr.IsMonochrome())
		require.Nil(t, internal.NewColors(pr).JSONPalette())
	})

	t.Run("colored_carries_prefixes", func(t *testing.T) {
		pr := output.NewPrinting()
		pr.EnableColor(true)
		require.False(t, pr.IsMonochrome())

		c := internal.NewColors(pr)
		pal := c.JSONPalette()
		require.NotNil(t, pal)

		cases := []struct {
			name string
			want internal.Color
			got  []byte
		}{
			{"Null", c.Null, pal.Null},
			{"Bool", c.Bool, pal.Bool},
			{"Number", c.Number, pal.Number},
			{"String", c.String, pal.String},
			{"Key", c.Key, pal.Key},
			{"Bytes", c.Bytes, pal.Bytes},
			{"Time", c.Time, pal.Time},
			{"Punc", c.Punc, pal.Punc},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				require.NotEmpty(t, tc.want.Prefix, "fixture should carry a prefix")
				require.Equal(t, string(tc.want.Prefix), string(tc.got),
					"palette field must be the Color's prefix")
				require.NotContains(t, string(tc.got), string(tc.want.Suffix),
					"jsoncolor emits its own reset, so the suffix must be discarded")
			})
		}
	})
}
