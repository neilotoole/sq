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
		{
			// Wrong scheme: not a mysql:// location.
			loc:     "postgres://sakila:p_ssW0rd@localhost:5432/sqtest",
			wantErr: true,
		},
		{
			// Too short: prefix matches but there's nothing after it.
			loc:     "mysql://",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
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

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "8.0.36-0ubuntu0.22.04.1", want: "v8.0.36"},
		{raw: "5.7.44", want: "v5.7.44"},
		{raw: "5.7", want: "v5.7.0"},
		{raw: "8.4.0", want: "v8.4.0"},
		{raw: "5.5.5-10.6.4-MariaDB", want: "v10.6.4"},                     // MariaDB replication sentinel
		{raw: "10.11.2-MariaDB-1:10.11.2+maria~ubu2204", want: "v10.11.2"}, // modern MariaDB, no sentinel
		{raw: "not-a-version", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseSemver(tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
