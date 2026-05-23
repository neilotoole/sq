package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParse_handleTableAlias_gh633 is a regression test for #633: an alias on
// the single-segment handleTable form (@handle.tbl:alias) was silently dropped
// by ANTLR error recovery, taking every downstream segment with it. After the
// fix the alias lands on the TblSelectorNode and the downstream segment(s)
// survive.
func TestParse_handleTableAlias_gh633(t *testing.T) {
	const input = `@sakila.actor:a | .a.first_name`
	a := mustParse(t, input)

	tbls := FindNodes[*TblSelectorNode](a)
	require.Len(t, tbls, 1)
	require.Equal(t, "@sakila", tbls[0].Handle())
	require.Equal(t, "actor", tbls[0].Table().Table)
	require.Equal(t, "a", tbls[0].Alias(),
		"alias on handleTable must be parsed, not dropped")

	// On master the downstream `.a.first_name` segment is dropped, collapsing
	// to a single segment. The fix preserves it (two segments, matching the
	// no-alias `@sakila.actor | .first_name` form).
	require.Len(t, a.Segments(), 2,
		"downstream segment after handleTable:alias must survive")
}

func TestExtractHandles(t *testing.T) {
	testCases := []struct {
		input string
		want  []string
	}{
		{
			input: "@sakila | .actor",
			want:  []string{"@sakila"},
		},
		{
			input: "@sakila_pg | .actor | join(@sakila_ms.film_actor, .actor_id)",
			want:  []string{"@sakila_ms", "@sakila_pg"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			a := mustParse(t, tc.input)
			got := ExtractHandles(a)
			require.Equal(t, tc.want, got)
		})
	}
}
