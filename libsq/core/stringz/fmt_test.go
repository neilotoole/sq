package stringz_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDecimal tests FormatDecimal, DecimalPlaces, and DecimalFloatOK.
// FormatDecimal trims trailing fractional zeros, so a decimal's value is
// rendered consistently regardless of the scale it was computed with. This
// matters for cross-driver aggregate output (e.g. sum()), where backends
// otherwise return the same value with different scales. DecimalPlaces still
// reports the scale derived from the decimal's exponent; the two are
// intentionally decoupled.
func TestDecimal(t *testing.T) {
	testCases := []struct {
		in          decimal.Decimal
		wantStr     string
		wantPlaces  int32
		wantFloatOK bool
	}{
		{in: decimal.New(0, 0), wantStr: "0", wantPlaces: 0, wantFloatOK: true},
		{in: decimal.New(0, -1), wantStr: "0", wantPlaces: 1, wantFloatOK: true},
		{in: decimal.New(0, -2), wantStr: "0", wantPlaces: 2, wantFloatOK: true},
		{in: decimal.New(0, 2), wantStr: "0", wantPlaces: 0, wantFloatOK: true},
		{in: decimal.NewFromFloat(1.1), wantStr: "1.1", wantPlaces: 1, wantFloatOK: true},
		{in: decimal.New(100, -2), wantStr: "1", wantPlaces: 2, wantFloatOK: true},
		{in: decimal.New(10000, -4), wantStr: "1", wantPlaces: 4, wantFloatOK: true},
		// Trailing zeros are trimmed (issue #839): the same sum value must
		// render identically no matter which scale a backend cast produced.
		{in: decimal.RequireFromString("20100.000000"), wantStr: "20100", wantPlaces: 6, wantFloatOK: true},
		{in: decimal.RequireFromString("67416.510000"), wantStr: "67416.51", wantPlaces: 6, wantFloatOK: true},
		{in: decimal.RequireFromString("100.50"), wantStr: "100.5", wantPlaces: 2, wantFloatOK: true},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i, tc.in, tc.wantStr), func(t *testing.T) {
			gotStr := stringz.FormatDecimal(tc.in)
			require.Equal(t, tc.wantStr, gotStr)
			gotPlaces := stringz.DecimalPlaces(tc.in)
			require.Equal(t, tc.wantPlaces, gotPlaces)
			gotFloatOK := stringz.DecimalFloatOK(tc.in)
			require.Equal(t, tc.wantFloatOK, gotFloatOK)
		})
	}
}

func TestParseBool(t *testing.T) {
	testCases := map[string]bool{
		"1":     true,
		"t":     true,
		"true":  true,
		"TRUE":  true,
		"y":     true,
		"Y":     true,
		"yes":   true,
		"Yes":   true,
		"YES":   true,
		"0":     false,
		"f":     false,
		"false": false,
		"False": false,
		"n":     false,
		"N":     false,
		"no":    false,
		"No":    false,
		"NO":    false,
	}

	for input, wantBool := range testCases {
		gotBool, gotErr := stringz.ParseBool(input)
		require.NoError(t, gotErr)
		require.Equal(t, wantBool, gotBool)
	}

	invalid := []string{"", " ", " true ", "gibberish", "-1"}
	for _, input := range invalid {
		_, gotErr := stringz.ParseBool(input)
		require.Error(t, gotErr)
	}
}

func TestFormatSize(t *testing.T) {
	t.Parallel()

	t.Run("nil_reports_dash", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "-", stringz.FormatSize(nil))
	})

	t.Run("zero_reports_zero_bytes", func(t *testing.T) {
		t.Parallel()
		var zero int64
		got := stringz.FormatSize(&zero)
		require.Equal(t, stringz.ByteSized(0, 1, ""), got)
	})

	t.Run("non_zero_matches_ByteSized", func(t *testing.T) {
		t.Parallel()
		v := int64(1048576)
		got := stringz.FormatSize(&v)
		require.Equal(t, stringz.ByteSized(v, 1, ""), got)
	})
}

func TestPlu(t *testing.T) {
	testCases := []struct {
		s    string
		i    int
		want string
	}{
		{s: "row(s)", i: 0, want: "rows"},
		{s: "row(s)", i: 1, want: "row"},
		{s: "row(s)", i: 2, want: "rows"},
		{s: "row(s) col(s)", i: 0, want: "rows cols"},
		{s: "row(s) col(s)", i: 1, want: "row col"},
		{s: "row(s) col(s)", i: 2, want: "rows cols"},
		{s: "row(s)", i: 2, want: "rows"},
		{s: "rows", i: 0, want: "rows"},
		{s: "rows", i: 1, want: "rows"},
		{s: "rows", i: 2, want: "rows"},
	}

	for _, tc := range testCases {
		got := stringz.Plu(tc.s, tc.i)
		require.Equal(t, tc.want, got)
	}
}
