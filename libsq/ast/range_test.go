package ast

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

// TestRowRange tests the row range mechanism.
//
//	[]       select all rows (no range)
//	[1]      select row[1]
//	[10:15]  select rows 10 thru 15
//	[0:15]   select rows 0 thru 15
//	[:15]    same as above (0 thru 15)
//	[10:]    select all rows from 10 onwards
func TestRowRange(t *testing.T) {
	testCases := []struct {
		in           string
		wantRowRange bool
		wantOffset   int
		wantLimit    int
	}{
		{".actor | .[]", false, 0, 0},
		{".actor | .[2]", true, 2, 1},
		{".actor | .[1:3]", true, 1, 2},
		{".actor | .[0:3]", true, 0, 3},
		{".actor | .[:3]", true, 0, 3},
		{".actor | .[2:]", true, 2, -1},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.in), func(t *testing.T) {
			ast := mustParse(t, tc.in)
			insp := NewInspector(ast)
			nodes := insp.FindNodes(typeRowRangeNode)

			if !tc.wantRowRange {
				require.Empty(t, nodes)
				return
			}

			require.Len(t, nodes, 1)
			rr, ok := nodes[0].(*RowRangeNode)
			require.True(t, ok)

			require.Equal(t, tc.wantOffset, rr.Offset)
			require.Equal(t, tc.wantLimit, rr.Limit)
		})
	}
}
