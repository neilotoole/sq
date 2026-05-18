package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenize_Simple(t *testing.T) {
	got := Tokenize(".actor | .first_name")
	require.NotEmpty(t, got)

	// Filter to just the kinds for compactness.
	kinds := make([]TokenKind, len(got))
	texts := make([]string, len(got))
	for i, tok := range got {
		kinds[i] = tok.Kind
		texts[i] = tok.Text
	}

	require.Equal(t, []string{".actor", "|", ".first_name"}, texts)
	require.Equal(t, []TokenKind{TokenName, TokenPunc, TokenName}, kinds)
}

func TestTokenize_Handle(t *testing.T) {
	got := Tokenize("@sakila/local/sl3 | .actor")
	require.NotEmpty(t, got)
	require.Equal(t, "@sakila/local/sl3", got[0].Text)
	require.Equal(t, TokenHandle, got[0].Kind)
}

func TestTokenize_Keyword(t *testing.T) {
	got := Tokenize(".actor | sum(.id)")
	// Find the "sum" token.
	var sumTok *Token
	for i := range got {
		if got[i].Text == "sum" {
			sumTok = &got[i]
			break
		}
	}
	require.NotNil(t, sumTok, "expected to find a 'sum' token")
	require.Equal(t, TokenKeyword, sumTok.Kind)
}

func TestTokenize_Number(t *testing.T) {
	got := Tokenize(".actor | .[0:3]")
	// Find numeric tokens.
	var numCount int
	for _, tok := range got {
		if tok.Kind == TokenNumber {
			numCount++
		}
	}
	require.GreaterOrEqual(t, numCount, 1, "expected at least one number token in '.[0:3]'")
}

func TestTokenize_String(t *testing.T) {
	got := Tokenize(`.actor | .first_name == "BOB"`)
	var foundString bool
	for _, tok := range got {
		if tok.Text == `"BOB"` && tok.Kind == TokenString {
			foundString = true
			break
		}
	}
	require.True(t, foundString, "expected '\"BOB\"' to be a TokenString")
}

func TestTokenize_RuneOffsets(t *testing.T) {
	input := ".actor | gibberish"
	got := Tokenize(input)
	require.Len(t, got, 3, "expected three visible tokens")
	require.Equal(t, ".actor", got[0].Text)
	require.Equal(t, 0, got[0].Start)
	require.Equal(t, 5, got[0].Stop) // inclusive: rune 5 is 'r' of ".actor"
	require.Equal(t, "|", got[1].Text)
	require.Equal(t, 7, got[1].Start)
	require.Equal(t, 7, got[1].Stop)
	require.Equal(t, "gibberish", got[2].Text)
	require.Equal(t, 9, got[2].Start)
	require.Equal(t, 17, got[2].Stop)
}

func TestTokenize_EmptyInput(t *testing.T) {
	got := Tokenize("")
	require.Empty(t, got)
}

func TestTokenKind_String(t *testing.T) {
	require.Equal(t, "handle", TokenHandle.String())
	require.Equal(t, "keyword", TokenKeyword.String())
	require.Equal(t, "unknown", TokenUnknown.String())
	require.Equal(t, "TokenKind(999)", TokenKind(999).String())
}
