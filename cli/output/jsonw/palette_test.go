package jsonw

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
)

// TestNewJSONColorPalette_NilOrMono returns nil for nil/monochrome
// Printing — the encoder treats nil Colors as "no colorization".
func TestNewJSONColorPalette_NilOrMono(t *testing.T) {
	require.Nil(t, newJSONColorPalette(nil))

	mono := output.NewPrinting()
	mono.EnableColor(false)
	require.True(t, mono.IsMonochrome())
	require.Nil(t, newJSONColorPalette(mono))
}

// TestNewJSONColorPalette_HasPrefixes verifies the adapter extracts the
// ANSI prefix bytes from each fatih/color field of Printing. We do not
// compare against hardcoded escape sequences — we compare against what
// fatih/color itself produces, so the test is robust to color changes
// in output.NewPrinting().
func TestNewJSONColorPalette_HasPrefixes(t *testing.T) {
	pr := output.NewPrinting()
	pr.EnableColor(true)
	require.False(t, pr.IsMonochrome())

	pal := newJSONColorPalette(pr)
	require.NotNil(t, pal)

	// Each field's Color (a []byte) should equal the prefix bytes that
	// fatih/color emits before " " when we round-trip a space through it.
	cases := []struct {
		name string
		fc   *color.Color
		got  []byte
	}{
		{"Null", pr.Null, pal.Null},
		{"Bool", pr.Bool, pal.Bool},
		{"Number", pr.Number, pal.Number},
		{"String", pr.String, pal.String},
		{"Key", pr.Key, pal.Key},
		{"Bytes", pr.Bytes, pal.Bytes},
		{"Time", pr.Datetime, pal.Time},
		{"Punc", pr.Punc, pal.Punc},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c2 := *tc.fc
			c2.EnableColor()
			rendered := []byte(c2.Sprint(" "))
			idx := bytes.IndexByte(rendered, ' ')
			require.Positive(t, idx, "fatih/color produced no prefix for %s", tc.name)
			wantPrefix := rendered[:idx]
			require.Equal(t, wantPrefix, tc.got)
		})
	}
}
