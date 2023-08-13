package loz_test

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/loz"
	"github.com/stretchr/testify/require"
)

func TestAll(t *testing.T) {
	gotAny := loz.All[any]()
	require.Equal(t, []any{}, gotAny)

	gotStrings := loz.All("hello", "world")
	require.Equal(t, []string{"hello", "world"}, gotStrings)
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

func TestHarmonizeSliceLengths(t *testing.T) {
	gotA, gotB := loz.HarmonizeSliceLengths(
		[]int{1, 2, 3},
		[]int{1, 2, 3, 4},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 7}, gotA)
	require.Equal(t, []int{1, 2, 3, 4}, gotB)

	gotA, gotB = loz.HarmonizeSliceLengths(
		[]int{1, 2, 3, 4},
		[]int{1, 2, 3},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 4}, gotA)
	require.Equal(t, []int{1, 2, 3, 7}, gotB)

	gotA, gotB = loz.HarmonizeSliceLengths(nil, nil, 7)
	require.Nil(t, gotA)
	require.Nil(t, gotB)

	gotA, gotB = loz.HarmonizeSliceLengths([]int{}, []int{}, 7)
	require.True(t, gotA != nil && len(gotA) == 0)
	require.True(t, gotB != nil && len(gotB) == 0)
}

func TestHarmonizeMatrixWidth(t *testing.T) {
	const defaultVal int = 7
	testCases := []struct {
		in   [][]int
		want [][]int
	}{
		//{nil, nil},
		//{[][]int{}, [][]int{}},
		//{[][]int{{1, 2, 3}}, [][]int{{1, 2, 3}}},
		{[][]int{{1, 2, 3}, {1, 2}}, [][]int{{1, 2, 3}, {1, 2, 7}}},
	}

	for i, tc := range testCases {
		t.Run(tutil.Name(i), func(t *testing.T) {
			loz.HarmonizeMatrixWidth(tc.in, defaultVal)
			require.EqualValues(t, tc.want, tc.in)
		})
	}
}
