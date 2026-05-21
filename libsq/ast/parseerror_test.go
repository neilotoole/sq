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
	require.Equal(t, "parser", first.stage)
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
	// '#' starts a line comment per the lexer; but mid-input it's
	// not recognized by any token rule.
	_, err := Parse(lg.Discard(), ".actor # bad")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	iss := pe.Issues[0]
	require.Equal(t, "lexer", iss.stage)
	require.Equal(t, "#", iss.Token, "lexer errors synthesize a single-char token at the error column")
	require.Equal(t, 7, iss.StartChar)
	require.Equal(t, 7, iss.StopChar)
	require.Equal(t, "unexpected '#'", iss.Msg)
}

func TestParse_SyntaxErrorMsg_LexerError_MultiLine(t *testing.T) {
	// '#' is unrecognized mid-input; here it sits on line 2. ANTLR reports
	// the error column as 0-based *within the line* (col 5), so the listener
	// must convert (line, col) to an absolute rune offset (12) when
	// synthesizing the span. A naive runes[col] lookup would mark the wrong
	// rune ('r' from line 1) and emit wrong start/stop offsets.
	//
	//	.actor\n.foo # bad
	//	0123456 789...   ^-- '#' is rune 12
	_, err := Parse(lg.Discard(), ".actor\n.foo # bad")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	iss := pe.Issues[0]
	require.Equal(t, "lexer", iss.stage)
	require.Equal(t, 2, iss.Line)
	require.Equal(t, 5, iss.Col, "0-based col within line 2")
	require.Equal(t, "#", iss.Token, "token synthesized at the absolute rune offset, not runes[col]")
	require.Equal(t, 12, iss.StartChar, "absolute rune offset of '#' in the full input")
	require.Equal(t, 12, iss.StopChar)
	require.Equal(t, "unexpected '#'", iss.Msg)
}

func TestRuneOffsetForLineCol(t *testing.T) {
	const input = ".actor\n.foo # bad\nx"
	runes := []rune(input)
	testCases := []struct {
		line, col int
		want      int
	}{
		{1, 0, 0},   // first rune
		{1, 6, 6},   // the '\n' position (col == line length)
		{2, 0, 7},   // first rune of line 2
		{2, 5, 12},  // the '#'
		{3, 0, 18},  // first rune of line 3 ('x')
		{0, 0, -1},  // invalid line
		{1, -1, -1}, // invalid col
		{99, 0, -1}, // line beyond input
	}
	for _, tc := range testCases {
		got := runeOffsetForLineCol(runes, tc.line, tc.col)
		require.Equal(t, tc.want, got, "line=%d col=%d", tc.line, tc.col)
	}
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

func TestParse_EOF_TruncatedInput(t *testing.T) {
	// Trailing pipe with no RHS triggers ANTLR's <EOF> offending token.
	_, err := Parse(lg.Discard(), ".actor |")
	require.Error(t, err)

	var pe *ParseError
	require.True(t, errors.As(err, &pe))
	require.NotEmpty(t, pe.Issues)
	iss := pe.Issues[0]
	require.Equal(t, "parser", iss.stage)
	require.Equal(t, "unexpected end of input", iss.Msg,
		"<EOF> offending token should yield the canonical 'unexpected end of input' message")
}
