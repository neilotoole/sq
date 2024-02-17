package stringz_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
)

// TestDecimal tests FormatDecimal, DecimalPlaces, and DecimalFloatOK.
// The FormatDecimal tests verifies that the function formats a decimal
// value as expected, especially that the number of decimal places matches
// the exponent of the decimal value.
func TestDecimal(t *testing.T) {
	testCases := []struct {
		in          decimal.Decimal
		wantStr     string
		wantPlaces  int32
		wantFloatOK bool
	}{
		{in: decimal.New(0, 0), wantStr: "0", wantPlaces: 0, wantFloatOK: true},
		{in: decimal.New(0, -1), wantStr: "0.0", wantPlaces: 1, wantFloatOK: true},
		{in: decimal.New(0, -2), wantStr: "0.00", wantPlaces: 2, wantFloatOK: true},
		{in: decimal.New(0, 2), wantStr: "0", wantPlaces: 0, wantFloatOK: true},
		{in: decimal.NewFromFloat(1.1), wantStr: "1.1", wantPlaces: 1, wantFloatOK: true},
		{in: decimal.New(100, -2), wantStr: "1.00", wantPlaces: 2, wantFloatOK: true},
		{in: decimal.New(10000, -4), wantStr: "1.0000", wantPlaces: 4, wantFloatOK: true},
	}

	for i, tc := range testCases {
		tc := tc
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
