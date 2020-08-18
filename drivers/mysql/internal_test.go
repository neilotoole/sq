package mysql

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/errz"
)

var KindFromDBTypeName = kindFromDBTypeName

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

func TestHasErrCode(t *testing.T) {
	var err error
	err = &mysql.MySQLError{
		Number:  1146,
		Message: "I'm not here",
	}

	require.True(t, hasErrCode(err, errNumTableNotExist))

	// Test that a wrapped error works
	err = errz.Err(err)
	require.True(t, hasErrCode(err, errNumTableNotExist))
}
