package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpectedTokenLiterals_StripsQuotes(t *testing.T) {
	literalNames := []string{"", "'sum'", "'avg'", "<INVALID>", "ARG", "'max'"}
	expected := []int{1, 2, 5} // sum, avg, max
	got := expectedTokenLiterals(expected, literalNames)
	require.Equal(t, []string{"sum", "avg", "max"}, got)
}

func TestExpectedTokenLiterals_SkipsSymbolicOnly(t *testing.T) {
	// Symbolic tokens (no quoted literal) aren't useful for suggestions.
	literalNames := []string{"", "'sum'", "", ""}
	expected := []int{1, 2, 3}
	got := expectedTokenLiterals(expected, literalNames)
	require.Equal(t, []string{"sum"}, got)
}
