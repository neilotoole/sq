package ast

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/testh/tu"
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
		t.Run(tu.Name(tc.in), func(t *testing.T) {
			t.Parallel()

			log := lgt.New(t)

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
