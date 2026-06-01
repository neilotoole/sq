package sqlite3

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

var (
	KindFromDBTypeName = kindFromDBTypeName
	GetTblRowCounts    = getTblRowCounts
	RTypeNullTime      = rtypeNullTime
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(?)"},
		{numCols: 2, numRows: 1, want: "(?, ?)"},
		{numCols: 1, numRows: 2, want: "(?), (?)"},
		{numCols: 2, numRows: 2, want: "(?, ?), (?, ?)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func TestDsnFromLocation(t *testing.T) {
	testCases := []struct {
		loc     string
		want    string
		wantErr bool
	}{
		{loc: "", wantErr: true},
		{loc: "duckdb://x", wantErr: true},
		{loc: Prefix + "/path/to/foo.db", want: "/path/to/foo.db"},
		{loc: Prefix + "/path/to/foo.db?mode=ro", want: "/path/to/foo.db?mode=ro"},
		{loc: Prefix + "/path/to/foo.db?cache=shared&mode=rw", want: "/path/to/foo.db?cache=shared&mode=rw"},
		{loc: Prefix + "/path/to/foo.db?immutable=1", want: "/path/to/foo.db?immutable=1"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got, err := dsnFromLocation(tc.loc)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
