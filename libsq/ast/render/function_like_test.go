package render

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEscapeLikePattern(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"foo", "foo"},
		{"50%", "50|%"},
		{"a_b", "a|_b"},
		{"a|b", "a||b"},
		{"%_|", "|%|_||"},
		{"O'Brien", "O'Brien"}, // single quote is NOT a LIKE meta-char
		{"100%_off|", "100|%|_off||"},
		{"café_au_lait", "café|_au|_lait"}, // multi-byte UTF-8 round-trips correctly
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			got := EscapeLikePattern(tc.in)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestBuildLikePattern(t *testing.T) {
	testCases := []struct {
		mode LikeMode
		in   string
		want string
	}{
		{LikeContains, "foo", "%foo%"},
		{LikeContains, "50%", "%50|%%"},
		{LikeStartsWith, "foo", "foo%"},
		{LikeStartsWith, "50%", "50|%%"},
		{LikeEndsWith, "foo", "%foo"},
		{LikeEndsWith, "_x", "%|_x"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.mode)+"/"+tc.in, func(t *testing.T) {
			got := BuildLikePattern(tc.in, tc.mode)
			require.Equal(t, tc.want, got)
		})
	}
}
