package yamlw_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/fatih/color"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/testh"
)

// TestRecordWriter_Color verifies that the YAML record writer colorizes values
// consistently with the JSON encoder, including numeric values in columns whose
// declared kind is kind.Unknown (e.g. aggregate results such as avg()). See #851.
func TestRecordWriter_Color(t *testing.T) {
	testCases := []struct {
		name    string
		k       kind.Kind
		val     any
		wantClr func(pr *output.Printing) *color.Color
		wantVal string
	}{
		// Unknown-kind columns resolve color from the Go value type.
		{"unknown_float", kind.Unknown, float64(100.5), func(pr *output.Printing) *color.Color { return pr.Number }, "100.5"},
		{"unknown_int", kind.Unknown, int64(42), func(pr *output.Printing) *color.Color { return pr.Number }, "42"},
		{
			"unknown_decimal",
			kind.Unknown,
			decimal.RequireFromString("100.5"),
			func(pr *output.Printing) *color.Color { return pr.Number },
			`"100.5"`,
		},
		{"unknown_bool", kind.Unknown, true, func(pr *output.Printing) *color.Color { return pr.Bool }, "true"},
		{"unknown_string", kind.Unknown, "hello", func(pr *output.Printing) *color.Color { return pr.String }, "hello"},

		// Columns with a concrete kind keep their kind-based color.
		{"int_kind", kind.Int, int64(42), func(pr *output.Printing) *color.Color { return pr.Number }, "42"},
		{"text_kind", kind.Text, "hello", func(pr *output.Printing) *color.Color { return pr.String }, "hello"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pr := output.NewPrinting()
			pr.EnableColor(true)

			got := renderColorYAML(t, pr, tc.k, tc.val)
			require.Contains(t, got, tc.wantClr(pr).Sprint(tc.wantVal))
		})
	}
}

// TestRecordWriter_Color_UnknownNotNormal guards against the #851 regression:
// a numeric value in a kind.Unknown column must not fall back to the Normal
// (no-color) path.
func TestRecordWriter_Color_UnknownNotNormal(t *testing.T) {
	pr := output.NewPrinting()
	pr.EnableColor(true)

	// The Number-colored form is the guard: had the value fallen back to the
	// Normal path it would be wrapped in pr.Normal's codes, not pr.Number's.
	got := renderColorYAML(t, pr, kind.Unknown, float64(100.5))
	require.Contains(t, got, pr.Number.Sprint("100.5"))
}

// TestRecordWriter_Color_Monochrome verifies no escape codes are emitted when
// color is disabled, including for the kind.Unknown value-type path.
func TestRecordWriter_Color_Monochrome(t *testing.T) {
	pr := output.NewPrinting()
	pr.EnableColor(false)

	got := renderColorYAML(t, pr, kind.Unknown, float64(100.5))
	require.Equal(t, "- c: 100.5\n", got)
	require.NotContains(t, got, "\x1b")
}

func renderColorYAML(t *testing.T, pr *output.Printing, k kind.Kind, val any) string {
	t.Helper()
	ctx := context.Background()
	recMeta := testh.NewRecordMeta([]string{"c"}, []kind.Kind{k})
	recs := []record.Record{{val}}

	buf := &bytes.Buffer{}
	w := yamlw.NewRecordWriter(buf, pr)
	require.NoError(t, w.Open(ctx, recMeta))
	require.NoError(t, w.WriteRecords(ctx, recs))
	require.NoError(t, w.Flush(ctx))
	require.NoError(t, w.Close(ctx))
	return buf.String()
}
