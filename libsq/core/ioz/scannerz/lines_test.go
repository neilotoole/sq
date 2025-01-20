package scannerz_test

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz/scannerz"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
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

	got := scannerz.IndentLines(context.Background(), input, "__")
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

	ctx := context.Background()

	require.Equal(t, -1, scannerz.LineCount(ctx, nil, true))

	for i, tc := range testCases {
		tc := tc

		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			count := scannerz.LineCount(ctx, strings.NewReader(tc.in), false)
			require.Equal(t, tc.withEmpty, count)
			count = scannerz.LineCount(ctx, strings.NewReader(tc.in), true)
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

	got := scannerz.VisitLines(context.Background(), input, func(i int, line string) string {
		return strconv.Itoa(i+1) + ". " + line + "<<"
	})

	require.Equal(t, want, got)
}

func TestTrimHead(t *testing.T) {
	require.Panics(t, func() {
		_ = scannerz.TrimHead(context.Background(), "a", -1)
	})

	testCases := []struct {
		in   string
		n    int
		want string
	}{
		{in: "", n: 0, want: ""},
		{in: "", n: 1, want: ""},
		{in: "a", n: 0, want: "a"},
		{in: "a\n", n: 0, want: "a\n"},
		{in: "a\nb", n: 0, want: "a\nb"},
		{in: "a\nb\n", n: 0, want: "a\nb\n"},
		{in: "a\nb\n", n: 1, want: "b\n"},
		{in: "a\nb\n", n: 2, want: ""},
		{in: "a\nb\n", n: 3, want: ""},
		{in: "a\nb\n", n: 0, want: "a\nb\n"},
		{in: "a\nb\nc\nd\ne\n", n: 3, want: "d\ne\n"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(tc.in, tc.n), func(t *testing.T) {
			got := scannerz.TrimHead(context.Background(), tc.in, tc.n)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestHead1(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "a", want: "a"},
		{in: "a\n", want: "a"},
		{in: "a\nb", want: "a"},
		{in: "a\nb\n", want: "a"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := scannerz.Head1(context.Background(), tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}
