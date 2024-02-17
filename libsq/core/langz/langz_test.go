package langz_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh/tu"
)

func TestAll(t *testing.T) {
	gotAny := langz.All[any]()
	require.Equal(t, []any{}, gotAny)

	gotStrings := langz.All("hello", "world")
	require.Equal(t, []string{"hello", "world"}, gotStrings)

	wantInts := []int{1, 2, 3}
	gotInts := langz.All(wantInts...)
	require.Equal(t, wantInts, gotInts)
	require.False(t, &gotInts == &wantInts,
		"wantInts and gotInts should not be the same slice")
}

func TestTypedSlice(t *testing.T) {
	bufs := []*bytes.Buffer{new(bytes.Buffer), new(bytes.Buffer)}

	var rdrs1, rdrs2 []io.Reader
	var ok bool
	rdrs1, ok = langz.TypedSlice[io.Reader](bufs...)
	require.True(t, ok)
	require.Len(t, rdrs1, 2)

	rdrs2 = langz.MustTypedSlice[io.Reader](bufs...)
	require.Len(t, rdrs2, 2)
	require.Equal(t, rdrs1, rdrs2)

	anys := []any{"hello", "world"}
	var strs []string
	strs, ok = langz.TypedSlice[string](anys...)
	require.True(t, ok)
	require.Len(t, strs, 2)
	require.Equal(t, []string{"hello", "world"}, strs)
}

func TestApply(t *testing.T) {
	input := []string{"hello", "world"}
	want := []string{"'hello'", "'world'"}
	got := langz.Apply(input, stringz.SingleQuote)
	require.Equal(t, want, got)
}

func TestAlignSliceLengths(t *testing.T) {
	gotA, gotB := langz.AlignSliceLengths(
		[]int{1, 2, 3},
		[]int{1, 2, 3, 4},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 7}, gotA)
	require.Equal(t, []int{1, 2, 3, 4}, gotB)

	gotA, gotB = langz.AlignSliceLengths(
		[]int{1, 2, 3, 4},
		[]int{1, 2, 3},
		7,
	)
	require.Equal(t, []int{1, 2, 3, 4}, gotA)
	require.Equal(t, []int{1, 2, 3, 7}, gotB)

	gotA, gotB = langz.AlignSliceLengths(nil, nil, 7)
	require.Nil(t, gotA)
	require.Nil(t, gotB)

	gotA, gotB = langz.AlignSliceLengths([]int{}, []int{}, 7)
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
			langz.AlignMatrixWidth(tc.in, defaultVal)
			require.EqualValues(t, tc.want, tc.in)
		})
	}
}

func TestIsSliceZeroed(t *testing.T) {
	require.True(t, langz.IsSliceZeroed([]any{}))
	require.True(t, langz.IsSliceZeroed[any](nil))
	require.True(t, langz.IsSliceZeroed([]int{0, 0}))
	require.False(t, langz.IsSliceZeroed([]int{0, 1}))
	require.True(t, langz.IsSliceZeroed([]string{"", ""}))
	require.False(t, langz.IsSliceZeroed([]string{"", "a"}))
}

func TestNewErrorAfterNReader_Read(t *testing.T) {
	const (
		errAfterN = 50
		bufSize   = 100
	)
	wantErr := errors.New("oh dear")

	b := make([]byte, bufSize)

	r := ioz.NewErrorAfterRandNReader(errAfterN, wantErr)
	n, err := r.Read(b)
	require.Error(t, err)
	require.True(t, errors.Is(err, wantErr))
	require.Equal(t, errAfterN, n)

	b = make([]byte, bufSize)
	n, err = r.Read(b)
	require.Error(t, err)
	require.True(t, errors.Is(err, wantErr))
	require.Equal(t, 0, n)
}

func TestNewErrorAfterNReader_ReadAll(t *testing.T) {
	const errAfterN = 50
	wantErr := errors.New("oh dear")

	r := ioz.NewErrorAfterRandNReader(errAfterN, wantErr)
	b, err := io.ReadAll(r)
	require.Error(t, err)
	require.True(t, errors.Is(err, wantErr))
	require.Equal(t, errAfterN, len(b))
}
