package loz_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
)

func TestAll(t *testing.T) {
	gotAny := loz.All[any]()
	require.Equal(t, []any{}, gotAny)

	gotStrings := loz.All("hello", "world")
	require.Equal(t, []string{"hello", "world"}, gotStrings)

	wantInts := []int{1, 2, 3}
	gotInts := loz.All(wantInts...)
	require.Equal(t, wantInts, gotInts)
	require.False(t, &gotInts == &wantInts,
		"wantInts and gotInts should not be the same slice")
}

func TestToSliceType(t *testing.T) {
	input1 := []any{"hello", "world"}

	var got []string
	var ok bool

	got, ok = loz.ToSliceType[any, string](input1...)
	require.True(t, ok)
	require.Len(t, got, 2)
	require.Equal(t, []string{"hello", "world"}, got)
}

func TestApply(t *testing.T) {
	input := []string{"hello", "world"}
	want := []string{"'hello'", "'world'"}
	got := loz.Apply(input, stringz.SingleQuote)
	require.Equal(t, want, got)
}

func TestAlignSliceLengths(t *testing.T) {
	gotA, gotB := loz.AlignSliceLengths(
		[]int{1, 2, 3},
		[]int{1, 2, 3, 4},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 7}, gotA)
	require.Equal(t, []int{1, 2, 3, 4}, gotB)

	gotA, gotB = loz.AlignSliceLengths(
		[]int{1, 2, 3, 4},
		[]int{1, 2, 3},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 4}, gotA)
	require.Equal(t, []int{1, 2, 3, 7}, gotB)

	gotA, gotB = loz.AlignSliceLengths(nil, nil, 7)
	require.Nil(t, gotA)
	require.Nil(t, gotB)

	gotA, gotB = loz.AlignSliceLengths([]int{}, []int{}, 7)
	require.True(t, gotA != nil && len(gotA) == 0)
	require.True(t, gotB != nil && len(gotB) == 0)
}

func TestAlignMatrixWidth(t *testing.T) {
	const defaultVal int = 7
	testCases := []struct {
		in   [][]int
		want [][]int
	}{
		{nil, nil},
		{[][]int{}, [][]int{}},
		{[][]int{{1, 2, 3}}, [][]int{{1, 2, 3}}},
		{[][]int{{1, 2, 3}, {1, 2}}, [][]int{{1, 2, 3}, {1, 2, 7}}},
	}

	for i, tc := range testCases {
		t.Run(tu.Name(i), func(t *testing.T) {
			loz.AlignMatrixWidth(tc.in, defaultVal)
			require.EqualValues(t, tc.want, tc.in)
		})
	}
}

func TestIsSliceZeroed(t *testing.T) {
	require.True(t, loz.IsSliceZeroed([]any{}))
	require.True(t, loz.IsSliceZeroed[any](nil))
	require.True(t, loz.IsSliceZeroed([]int{0, 0}))
	require.False(t, loz.IsSliceZeroed([]int{0, 1}))
	require.True(t, loz.IsSliceZeroed([]string{"", ""}))
	require.False(t, loz.IsSliceZeroed([]string{"", "a"}))
}
