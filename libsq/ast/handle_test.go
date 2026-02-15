package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
