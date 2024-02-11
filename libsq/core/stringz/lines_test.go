package stringz_test

import (
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
	"github.com/stretchr/testify/require"
	"strconv"
	"strings"
	"testing"
)

func TestIndentLines(t *testing.T) {
	const input = `In Xanadu did
Kubla Khan a stately
pleasure dome decree.

`
	const want = `__In Xanadu did
__Kubla Khan a stately
__pleasure dome decree.
__`

	got := stringz.IndentLines(input, "__")
	require.Equal(t, got, want)
}

func TestLineCount(t *testing.T) {
	testCases := []struct {
		in        string
		withEmpty int
		skipEmpty int
	}{
		{in: "", withEmpty: 0, skipEmpty: 0},
		{in: "\n", withEmpty: 1, skipEmpty: 0},
		{in: "\n\n", withEmpty: 2, skipEmpty: 0},
		{in: "\n\n", withEmpty: 2, skipEmpty: 0},
		{in: " ", withEmpty: 1, skipEmpty: 1},
		{in: "one", withEmpty: 1, skipEmpty: 1},
		{in: "one\n", withEmpty: 1, skipEmpty: 1},
		{in: "\none\n", withEmpty: 2, skipEmpty: 1},
		{in: "one\ntwo", withEmpty: 2, skipEmpty: 2},
		{in: "one\ntwo\n", withEmpty: 2, skipEmpty: 2},
		{in: "one\ntwo\n ", withEmpty: 3, skipEmpty: 3},
		{in: "one\n\nthree", withEmpty: 3, skipEmpty: 2},
		{in: "one\n\nthree\n", withEmpty: 3, skipEmpty: 2},
	}

	require.Equal(t, -1, stringz.LineCount(nil, true))

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			count := stringz.LineCount(strings.NewReader(tc.in), false)
			require.Equal(t, tc.withEmpty, count)
			count = stringz.LineCount(strings.NewReader(tc.in), true)
			require.Equal(t, tc.skipEmpty, count)
		})
	}
}

func TestVisitLines(t *testing.T) {
	const input = `In Xanadu did
Kubla Khan a stately
pleasure dome decree.

`
	const want = `1. In Xanadu did<<
2. Kubla Khan a stately<<
3. pleasure dome decree.<<
4. <<`

	got := stringz.VisitLines(input, func(i int, line string) string {
		return strconv.Itoa(i+1) + ". " + line + "<<"
	})

	require.Equal(t, want, got)
}
