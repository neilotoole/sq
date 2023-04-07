package ast

import (
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"
)

func TestColumnAlias(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in          string
		wantErr     bool
		wantColName string
		wantAlias   string
	}{
		{
			in:          `@sakila | .actor | .first_name:given_name`,
			wantColName: "first_name",
			wantAlias:   "given_name",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(tc.in), func(t *testing.T) {
			t.Parallel()

			log := slogt.New(t)

			ast, err := Parse(log, tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			insp := NewInspector(ast)
			nodes := insp.FindNodes(typeColSelectorNode)
			require.Equal(t, 1, len(nodes))
			colSel, ok := nodes[0].(*ColSelectorNode)
			require.True(t, ok)

			require.Equal(t, tc.wantColName, colSel.ColName())
			require.Equal(t, tc.wantAlias, colSel.Alias())
		})
	}
}
