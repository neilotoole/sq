package stringz_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/require"
)

func TestDoubleQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: ``, want: `""`},
		{in: `"hello"`, want: `"""hello"""`},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.DoubleQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestStripDoubleQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: ``, want: ``},
		{in: `"`, want: `"`},
		{in: `""`, want: ``},
		{in: `"a`, want: `"a`},
		{in: `"a"`, want: `a`},
		{in: `"abc"`, want: `abc`},
		{in: `"hello "" world"`, want: `hello "" world`},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			got := stringz.StripDoubleQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBacktickQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: "``"},
		{in: "`world`", want: "```world```"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.BacktickQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSingleQuote(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: "''"},
		{in: "jessie's girl", want: "'jessie''s girl'"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := stringz.SingleQuote(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
