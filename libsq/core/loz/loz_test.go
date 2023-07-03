package loz_test

import (
	"testing"

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
