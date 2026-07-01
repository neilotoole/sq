package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

// Export for testing.
var (
	GetTableColumnNames   = getTableColumnNames
	IsErrRelationNotExist = isErrRelationNotExist
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "($1)"},
		{numCols: 2, numRows: 1, want: "($1, $2)"},
		{numCols: 1, numRows: 2, want: "($1), ($2)"},
		{numCols: 2, numRows: 2, want: "($1, $2), ($3, $4)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}

func TestReplacePlaceholders(t *testing.T) {
	testCases := map[string]string{
		"":                "",
		"hello":           "hello",
		"?":               "$1",
		"??":              "$1$2",
		" ? ":             " $1 ",
		"(?, ?)":          "($1, $2)",
		"(?, ?, ?)":       "($1, $2, $3)",
		" (?  , ? , ?)  ": " ($1  , $2 , $3)  ",
	}

	for input, want := range testCases {
		got := replacePlaceholders(input)
		require.Equal(t, want, got)
	}
}

func Test_idSanitize(t *testing.T) {
	testCases := map[string]string{
		`tbl_name`: `"tbl_name"`,
	}

	for input, want := range testCases {
		got := idSanitize(input)
		require.Equal(t, want, got)
	}
}

func TestIsErrTooManyConnections(t *testing.T) {
	var err error

	err = &pgconn.PgError{Code: "53300"}
	require.True(t, isErrTooManyConnections(err))

	// Test with a wrapped error
	err = errw(err)
	require.True(t, isErrTooManyConnections(err))
}

func TestParseSemver(t *testing.T) {
	testCases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "16.1", want: "v16.1.0"},                             // PG 10+ is two-part; Canonical pads
		{raw: "16.1 (Ubuntu 16.1-1.pgdg22.04+1)", want: "v16.1.0"}, // distro parenthetical after token
		{raw: "9.6.24", want: "v9.6.24"},                           // pre-10 three-part
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
