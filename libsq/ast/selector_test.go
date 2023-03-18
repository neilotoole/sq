package ast

import (
	"testing"

	"github.com/neilotoole/sq/testh/tutil"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/lg/testlg"
)

func TestColumnAlias(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in        string
		wantErr   bool
		wantExpr  string
		wantAlias string
	}{
		{
			in:        `@sakila | .actor | .first_name:given_name`,
			wantExpr:  "first_name",
			wantAlias: "given_name",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tutil.Name(tc.in), func(t *testing.T) {
			t.Parallel()

			log := testlg.New(t)

			ast, err := Parse(log, tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			insp := NewInspector(log, ast)
			nodes := insp.FindNodes(typeColSelector)
			require.Equal(t, 1, len(nodes))
			colSel, ok := nodes[0].(*ColSelector)
			require.True(t, ok)
			expr, _ := colSel.ColExpr()

			require.Equal(t, tc.wantExpr, expr)
			require.Equal(t, tc.wantAlias, colSel.Alias())
		})
	}
}
