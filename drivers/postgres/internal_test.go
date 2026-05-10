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

func TestResolveTableColumnsFold(t *testing.T) {
	actual := []string{"actor_id", "first_name", "last_name", "MixedCase"}

	testCases := []struct {
		name    string
		src     []string
		want    []string
		wantErr string
	}{
		{
			name: "exact_lowercase",
			src:  []string{"actor_id", "first_name"},
			want: []string{"actor_id", "first_name"},
		},
		{
			name: "uppercase_from_oracle",
			src:  []string{"ACTOR_ID", "FIRST_NAME", "LAST_NAME"},
			want: []string{"actor_id", "first_name", "last_name"},
		},
		{
			name: "preserves_input_order",
			src:  []string{"LAST_NAME", "ACTOR_ID"},
			want: []string{"last_name", "actor_id"},
		},
		{
			name: "mixed_case_canonical_returned",
			src:  []string{"mixedcase"},
			want: []string{"MixedCase"},
		},
		{
			name: "empty_input_yields_empty",
			src:  []string{},
			want: []string{},
		},
		{
			name:    "missing_column_errors",
			src:     []string{"ACTOR_ID", "BOGUS_COL"},
			wantErr: `column "BOGUS_COL" does not exist`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTableColumnsFold(actual, tc.src)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
