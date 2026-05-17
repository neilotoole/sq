package ast

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg"
)

func TestParse_ParseErrorPopulated(t *testing.T) {
	input := ".actor | this_is_invalid(.first_name)"
	_, err := Parse(lg.Discard(), input)
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe), "expected *ParseError, got %T", err)
	require.Equal(t, input, pe.Input)
	require.NotEmpty(t, pe.Issues)

	first := pe.First()
	require.NotNil(t, first)
	require.Equal(t, "parser", first.Stage)
	require.Equal(t, 1, first.Line)
	require.Equal(t, 9, first.Col, "0-based col of 'this_is_invalid' in input")
	require.Equal(t, "this_is_invalid", first.Token)
	require.Equal(t, 9, first.StartChar)
	require.Equal(t, 23, first.StopChar, "inclusive end of 'this_is_invalid'")
}
