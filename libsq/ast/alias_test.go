package ast

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/testh/tu"
)

// collectAliases returns every non-empty alias attached to a result-column,
// table, expression, or function node in ast.
func collectAliases(ast *AST) []string {
	var aliases []string
	add := func(s string) {
		if s != "" {
			aliases = append(aliases, s)
		}
	}
	for _, n := range FindNodes[*ColSelectorNode](ast) {
		add(n.Alias())
	}
	for _, n := range FindNodes[*TblColSelectorNode](ast) {
		add(n.Alias())
	}
	for _, n := range FindNodes[*TblSelectorNode](ast) {
		add(n.Alias())
	}
	// Join target tables hang off JoinNode.Table() rather than being tree
	// children, so they aren't reached by the TblSelectorNode walk above.
	for _, n := range FindNodes[*JoinNode](ast) {
		if tbl := n.Table(); tbl != nil {
			add(tbl.Alias())
		}
	}
	for _, n := range FindNodes[*ExprElementNode](ast) {
		add(n.Alias())
	}
	for _, n := range FindNodes[*FuncNode](ast) {
		add(n.Alias())
	}
	return aliases
}

// TestAlias_ReservedWordApplied verifies that an alias which is a reserved
// word (the ALIAS_RESERVED token, e.g. ":count") is applied as the alias,
// not silently dropped, in every alias position. See issue #646.
func TestAlias_ReservedWordApplied(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   string
	}{
		{"col-selector", `@sakila | .actor | .first_name:count`},
		{"tbl-selector-multiseg", `@sakila | .actor:count | .first_name`},
		{"handle-table", `@sakila.actor:count | .first_name`},
		{"join-table", `@sakila | .actor | join(.film_actor:count, .actor.actor_id == .film_actor.actor_id)`},
		{"expr-element", `@sakila | .actor | (1+2):count`},
		{"func", `@sakila | .actor | sum(.actor_id):count`},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.name), func(t *testing.T) {
			t.Parallel()

			log := lgt.New(t)
			ast, err := Parse(log, tc.in)
			require.NoError(t, err)
			require.Contains(t, collectAliases(ast), "count",
				"alias %q should be applied for %q", "count", tc.in)
		})
	}
}

// TestAlias_ArgRejected verifies that an argument reference used as an alias
// (the ARG token, e.g. ":$x") is rejected with an error rather than silently
// dropped or applied as a literal "$x" column. See issue #646.
func TestAlias_ArgRejected(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		in   string
	}{
		{"col-selector", `@sakila | .actor | .first_name:$x`},
		{"tbl-selector-multiseg", `@sakila | .actor:$x | .first_name`},
		{"handle-table", `@sakila.actor:$x | .first_name`},
		{"join-table", `@sakila | .actor | join(.film_actor:$x, .actor.actor_id == .film_actor.actor_id)`},
		{"expr-element", `@sakila | .actor | (1+2):$x`},
		{"func", `@sakila | .actor | sum(.actor_id):$x`},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.name), func(t *testing.T) {
			t.Parallel()

			log := lgt.New(t)
			_, err := Parse(log, tc.in)
			require.Error(t, err, "arg alias should be rejected for %q", tc.in)
		})
	}
}
