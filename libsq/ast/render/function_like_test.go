package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapeLikePattern(t *testing.T) {
	testCases := []struct {
		in        string
		extraMeta string
		want      string
	}{
		{"", "", ""},
		{"foo", "", "foo"},
		{"50%", "", "50|%"},
		{"a_b", "", "a|_b"},
		{"a|b", "", "a||b"},
		{"%_|", "", "|%|_||"},
		{"O'Brien", "", "O'Brien"}, // single quote is NOT a LIKE meta-char
		{"100%_off|", "", "100|%|_off||"},
		{"café_au_lait", "", "café|_au|_lait"}, // multi-byte UTF-8 round-trips correctly

		// SQL Server-style extraMeta: `[` and `]` are LIKE meta-chars and
		// must be escaped to match a literal substring (e.g. "[A-Z]"
		// should match the 5-char literal, not the character class).
		{"[A-Z]", "[]", "|[A-Z|]"},
		{"a[b]c", "[]", "a|[b|]c"},
		{"]start", "[]", "|]start"}, // `]` outside a class is literal in SQL Server but we escape defensively
		{"plain", "[]", "plain"},    // extraMeta doesn't trigger when no special chars are present
		{"50%[x]", "[]", "50|%|[x|]"},
	}

	for _, tc := range testCases {
		t.Run(tc.in+"/extra="+tc.extraMeta, func(t *testing.T) {
			got := escapeLikePattern(tc.in, tc.extraMeta)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBuildLikePattern(t *testing.T) {
	testCases := []struct {
		mode      LikeMode
		in        string
		extraMeta string
		want      string
	}{
		{LikeContains, "foo", "", "%foo%"},
		{LikeContains, "50%", "", "%50|%%"},
		{LikeStartsWith, "foo", "", "foo%"},
		{LikeStartsWith, "50%", "", "50|%%"},
		{LikeEndsWith, "foo", "", "%foo"},
		{LikeEndsWith, "_x", "", "%|_x"},

		// SQL Server-style extraMeta: bracket escaping is folded into the
		// LIKE-mode wrapping.
		{LikeContains, "[A-Z]", "[]", "%|[A-Z|]%"},
		{LikeStartsWith, "[x", "[]", "|[x%"},
		{LikeEndsWith, "y]", "[]", "%y|]"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.mode)+"/"+tc.in+"/extra="+tc.extraMeta, func(t *testing.T) {
			got := buildLikePattern(tc.in, tc.mode, tc.extraMeta)
			require.Equal(t, tc.want, got)
		})
	}
}
