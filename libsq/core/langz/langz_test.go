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

	// Empty input yields an empty (non-nil) slice and ok=true.
	got, ok := langz.TypedSlice[string, any]()
	require.True(t, ok)
	require.NotNil(t, got)
	require.Empty(t, got)

	// A failed conversion returns nil, false.
	mixed := []any{"hello", 42}
	got, ok = langz.TypedSlice[string](mixed...)
	require.False(t, ok)
	require.Nil(t, got)
}

func TestMustTypedSlice(t *testing.T) {
	bufs := []*bytes.Buffer{new(bytes.Buffer), new(bytes.Buffer)}
	rdrs := langz.MustTypedSlice[io.Reader](bufs...)
	require.Len(t, rdrs, 2)

	require.Panics(t, func() {
		mixed := []any{"hello", 42}
		_ = langz.MustTypedSlice[string](mixed...)
	}, "should panic when a conversion fails")
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

func TestJoinSlices(t *testing.T) {
	// Always returns a new, non-nil slice, even for all-nil input.
	got := langz.JoinSlices[int](nil)
	require.NotNil(t, got)
	require.Empty(t, got)

	got = langz.JoinSlices[int](nil, nil, nil)
	require.NotNil(t, got)
	require.Empty(t, got)

	// Order is preserved across the input slices.
	got = langz.JoinSlices([]int{1, 2}, []int{3}, []int{4, 5})
	require.Equal(t, []int{1, 2, 3, 4, 5}, got)

	// No others; result is a copy, not the same backing array.
	a := []int{1, 2, 3}
	got = langz.JoinSlices(a)
	require.Equal(t, a, got)
	got[0] = 99
	require.Equal(t, []int{1, 2, 3}, a, "input slice must not be modified")
}

func TestMake(t *testing.T) {
	require.Equal(t, []int{}, langz.Make(0, 7))
	require.Equal(t, []int{7, 7, 7}, langz.Make(3, 7))
	require.Equal(t, []string{"x", "x"}, langz.Make(2, "x"))
}

func TestNilIfZero(t *testing.T) {
	require.Nil(t, langz.NilIfZero(0))
	require.Nil(t, langz.NilIfZero(""))

	got := langz.NilIfZero(42)
	require.NotNil(t, got)
	require.Equal(t, 42, *got)

	gotStr := langz.NilIfZero("hello")
	require.NotNil(t, gotStr)
	require.Equal(t, "hello", *gotStr)
}

func TestZeroIfNil(t *testing.T) {
	require.Equal(t, 0, langz.ZeroIfNil[int](nil))
	require.Equal(t, "", langz.ZeroIfNil[string](nil))

	v := 42
	require.Equal(t, 42, langz.ZeroIfNil(&v))
	s := "hello"
	require.Equal(t, "hello", langz.ZeroIfNil(&s))
}

func TestTake(t *testing.T) {
	// Nil channel returns false.
	var nilCh chan struct{}
	require.False(t, langz.Take(nilCh))

	// Open channel with no value available returns false.
	openCh := make(chan struct{})
	require.False(t, langz.Take(openCh))

	// Buffered channel with a value available returns true.
	bufCh := make(chan int, 1)
	bufCh <- 1
	require.True(t, langz.Take(bufCh))
	// Value was consumed, so a second take returns false.
	require.False(t, langz.Take(bufCh))

	// Closed channel always returns true.
	closedCh := make(chan struct{})
	close(closedCh)
	require.True(t, langz.Take(closedCh))
}

func TestRemove(t *testing.T) {
	// Distinct underlying values so testify's reflect.DeepEqual-based
	// matchers don't treat the pointers as equal.
	v1, v2, v3, v4 := 1, 2, 3, 4
	a, b, c, d := &v1, &v2, &v3, &v4

	// Removes the element, preserving order.
	got := langz.Remove([]*int{a, b, c}, b)
	require.Equal(t, []*int{a, c}, got)

	// Element not present: slice returned unchanged.
	got = langz.Remove([]*int{a, c}, d)
	require.Equal(t, []*int{a, c}, got)

	// Empty slice.
	got = langz.Remove([]*int{}, a)
	require.Empty(t, got)
}

func TestRemoveUnordered(t *testing.T) {
	v1, v2, v3, v4 := 1, 2, 3, 4
	a, b, c, d := &v1, &v2, &v3, &v4

	// Removes the element (order not guaranteed).
	got := langz.RemoveUnordered([]*int{a, b, c}, b)
	require.Len(t, got, 2)
	require.NotContains(t, got, b)
	require.Contains(t, got, a)
	require.Contains(t, got, c)

	// Element not present: slice returned unchanged.
	got = langz.RemoveUnordered([]*int{a, c}, d)
	require.Equal(t, []*int{a, c}, got)

	// Empty slice.
	got = langz.RemoveUnordered([]*int{}, a)
	require.Empty(t, got)
}

func TestCond(t *testing.T) {
	require.Equal(t, "yes", langz.Cond(true, "yes", "no"))
	require.Equal(t, "no", langz.Cond(false, "yes", "no"))
	require.Equal(t, 1, langz.Cond(true, 1, 2))
	require.Equal(t, 2, langz.Cond(false, 1, 2))
}

func TestNonEmptyOf(t *testing.T) {
	require.Equal(t, "a", langz.NonEmptyOf("a", "b"))
	require.Equal(t, "b", langz.NonEmptyOf("", "b"))
	require.Equal(t, 1, langz.NonEmptyOf(1, 2))
	require.Equal(t, 2, langz.NonEmptyOf(0, 2))
	// Both zero: returns b (also zero).
	require.Equal(t, 0, langz.NonEmptyOf(0, 0))
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
