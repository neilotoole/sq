package ast

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/testh/tu"
)

// TestParse_OrderBy_SelectorError checks that a malformed selector inside
// order_by() surfaces the same build error it does anywhere else, rather than
// being silently dropped. VisitOrderByTerm used to discard the error from
// newSelectorNode (returning nil), so the ordering term vanished and the query
// ran unordered with no diagnostic.
func TestParse_OrderBy_SelectorError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in      string
		wantErr bool
	}{
		{in: `@sakila | .actor | order_by(.first_name)`, wantErr: false},
		{in: `@sakila | .actor | order_by(.first_name+)`, wantErr: false},
		{in: `@sakila | .actor | order_by(.actor.first_name)`, wantErr: false},
		// `.""` is rejected by newSelectorNode as too short; a plain selector
		// `.actor | .""` already errors, and order_by must behave the same.
		{in: `@sakila | .actor | order_by(."")`, wantErr: true},
		{in: `@sakila | .actor | order_by(."", .first_name)`, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.in), func(t *testing.T) {
			t.Parallel()

			_, err := Parse(lgt.New(t), tc.in)
			if tc.wantErr {
				require.Error(t, err, "malformed order_by selector should not be swallowed")
				return
			}
			require.NoError(t, err)
		})
	}
}
