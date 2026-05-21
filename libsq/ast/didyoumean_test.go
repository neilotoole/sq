package ast

import (
	"strings"
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

func TestExpectedTokenLiterals_SkipsPunctuation(t *testing.T) {
	// Operator/punctuation literals aren't useful did-you-mean targets;
	// only alphabetic-word literals (keywords, function names) are kept.
	literalNames := []string{"", "'sum'", "'|'", "'=='", "'where'", "'('"}
	expected := []int{1, 2, 3, 4, 5}
	got := expectedTokenLiterals(expected, literalNames)
	require.Equal(t, []string{"sum", "where"}, got)
}

func TestSuggestForToken(t *testing.T) {
	candidates := []string{
		"sum", "avg", "max", "min", "count", "contains",
		"icontains", "startswith", "endswith",
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"close-typo-max", "mx", "max"},
		{"close-typo-count", "cont", "count"},
		{"close-typo-contains", "containz", "contains"},
		{"exact-match-not-suggested", "max", ""}, // not a typo
		{"too-far", "this_is_invalid", ""},
		{"empty", "", ""},
		// A pathologically long token must be cheaply rejected (rune-length
		// difference far exceeds the threshold), not run through Levenshtein.
		{"pathologically-long", strings.Repeat("z", 4096), ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := suggestForToken(tc.in, candidates)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMaxEditDistance(t *testing.T) {
	require.Equal(t, 1, maxEditDistance(1))
	require.Equal(t, 1, maxEditDistance(4))
	require.Equal(t, 2, maxEditDistance(5))
	require.Equal(t, 2, maxEditDistance(9))
	require.Equal(t, 3, maxEditDistance(10))
	require.Equal(t, 3, maxEditDistance(50))
}

func TestCollectExpectedTokenTypes(t *testing.T) {
	tests := []struct {
		name string
		in   [][2]int
		want []int
	}{
		{"nil", nil, nil},
		{"empty", [][2]int{}, nil},
		{"single-element", [][2]int{{3, 3}}, []int{3}},
		{"range", [][2]int{{3, 5}}, []int{3, 4, 5}},
		{"multiple", [][2]int{{1, 1}, {3, 5}, {10, 10}}, []int{1, 3, 4, 5, 10}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, collectExpectedTokenTypes(tc.in))
		})
	}
}
