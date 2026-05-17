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

func TestParse_SyntaxErrorMsg_Terse(t *testing.T) {
	_, err := Parse(lg.Discard(), ".actor | this_is_invalid(.first_name)")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)

	got := pe.Issues[0].Msg
	require.Equal(t, "unexpected 'this_is_invalid'", got)
	require.NotContains(t, got, "expecting", "should not include ANTLR's expected-token dump")
}

func TestParse_SyntaxErrorMsg_LexerError(t *testing.T) {
	// '#' is not a valid lexer character anywhere in SLQ.
	_, err := Parse(lg.Discard(), ".actor # bad")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	iss := pe.Issues[0]
	require.Equal(t, "lexer", iss.Stage)
	require.Empty(t, iss.Token, "lexer errors produce no offending token")
	require.Equal(t, -1, iss.StartChar, "lexer errors have no char span")
	require.Equal(t, -1, iss.StopChar)
	require.Equal(t, "unexpected '# bad'", iss.Msg)
}

func TestParse_DidYouMean(t *testing.T) {
	// "mx" should suggest "max" because both are short and edit distance 1.
	_, err := Parse(lg.Discard(), ".actor | mx(.id)")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	require.Equal(t, "max", pe.Issues[0].Suggestion)
}

func TestParse_NoSuggestionForFarToken(t *testing.T) {
	_, err := Parse(lg.Discard(), ".actor | this_is_invalid(.id)")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	require.Empty(t, pe.Issues[0].Suggestion, "no close match should produce no suggestion")
}
