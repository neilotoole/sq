package sqlserver

import (
	"fmt"
	"testing"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/require"
)

func Test_placeholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(@p1)"},
		{numCols: 2, numRows: 1, want: "(@p1, @p2)"},
		{numCols: 1, numRows: 2, want: "(@p1), (@p2)"},
		{numCols: 2, numRows: 2, want: "(@p1, @p2), (@p3, @p4)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func Test_hasErrCode(t *testing.T) {
	const wantCode = 100
	var err error

	require.False(t, hasErrCode(nil, wantCode))
	err = fmt.Errorf("huzzah")
	require.False(t, hasErrCode(err, wantCode))

	err = mssql.Error{
		Number: wantCode,
	}

	require.True(t, hasErrCode(err, wantCode))
}
