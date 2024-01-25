//nolint:lll
package mysql

import (
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

// Export for testing.
var (
	KindFromDBTypeName     = kindFromDBTypeName
	GetTableRowCountsBatch = getTableRowCountsBatch
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

func TestHasErrCode(t *testing.T) {
	var err error
	err = &mysql.MySQLError{
		Number:  1146,
		Message: "I'm not here",
	}

	require.True(t, hasErrCode(err, errNumTableNotExist))

	// Test that a wrapped error works
	err = errw(err)
	require.True(t, hasErrCode(err, errNumTableNotExist))
}

func TestDSNFromLocation(t *testing.T) {
	testCases := []struct {
		loc       string
		parseTime bool
		wantDSN   string
		wantErr   bool
	}{
		{
			loc:     "mysql://sakila:p_ssW0rd@localhost:3306/sqtest",
			wantDSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest",
		},
		{
			loc:       "mysql://sakila:p_ssW0rd@localhost:3306/sqtest",
			wantDSN:   "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?parseTime=true",
			parseTime: true,
		},
		{
			loc:       "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?parseTime=true",
			wantDSN:   "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?parseTime=true",
			parseTime: true,
		},
		{
			loc:     "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowOldPasswords=true",
			wantDSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowOldPasswords=true",
		},
		{
			loc:       "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowOldPasswords=true",
			wantDSN:   "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowOldPasswords=true&parseTime=true",
			parseTime: true,
		},
		{
			loc:     "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowOldPasswords=true",
			wantDSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowOldPasswords=true",
		},
		{
			loc:     "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowCleartextPasswords=true&allowOldPasswords=true",
			wantDSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowCleartextPasswords=true&allowOldPasswords=true",
		},
		{
			loc:       "mysql://sakila:p_ssW0rd@localhost:3306/sqtest?parseTime=true&allowCleartextPasswords=true&allowOldPasswords=true",
			wantDSN:   "sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowCleartextPasswords=true&allowOldPasswords=true&parseTime=true",
			parseTime: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tu.Name(tc.loc, tc.parseTime), func(t *testing.T) {
			src := &source.Source{
				Handle:   "@testhandle",
				Type:     drivertype.MySQL,
				Location: tc.loc,
			}

			gotDSN, gotErr := dsnFromLocation(src, tc.parseTime)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.wantDSN, gotDSN)
		})
	}
}
