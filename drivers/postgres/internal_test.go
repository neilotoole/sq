package postgres

import (
	"context"
	"errors"
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

// TestIsErrScanRetryable pins the retry predicate used by the bulk metadata
// loaders (loadWithRetry / doRetryVanished): too-many-connections, a relation
// that no longer exists, and the XX000 internal error raised when a relation
// is dropped between OID resolution and access are retryable; anything else
// is not. See issue #1025.
func TestIsErrScanRetryable(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "relation_not_exist", err: &pgconn.PgError{Code: errCodeRelationNotExist}, want: true},
		{name: "too_many_connections", err: &pgconn.PgError{Code: errCodeTooManyConnections}, want: true},
		{name: "internal_error", err: &pgconn.PgError{Code: errCodeInternalError}, want: true},
		// The predicate sees errors that have already been wrapped via errw, so
		// each retryable code is also covered in wrapped form.
		{name: "wrapped_relation_not_exist", err: errw(&pgconn.PgError{Code: errCodeRelationNotExist}), want: true},
		{name: "wrapped_too_many_connections", err: errw(&pgconn.PgError{Code: errCodeTooManyConnections}), want: true},
		{name: "wrapped_internal_error", err: errw(&pgconn.PgError{Code: errCodeInternalError}), want: true},
		{name: "syntax_error", err: &pgconn.PgError{Code: "42601"}, want: false},
		{name: "non_pg_error", err: errors.New("boom"), want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isErrScanRetryable(tc.err))
		})
	}
}

// TestLoadWithRetry_VanishedRelation pins the retry behavior of the bulk
// metadata loaders: a transient vanished-relation error (a relation dropped
// mid-scan by concurrent DDL) is retried and resolves on the next attempt,
// while a non-retryable error surfaces immediately without a retry.
func TestLoadWithRetry_VanishedRelation(t *testing.T) {
	t.Parallel()

	t.Run("transient_vanished_relation", func(t *testing.T) {
		t.Parallel()
		var calls int
		got, err := loadWithRetry(context.Background(), func() (int, error) {
			calls++
			if calls == 1 {
				return 0, errw(&pgconn.PgError{
					Code:    errCodeInternalError,
					Message: "could not open relation with OID 12345",
				})
			}
			return 7, nil
		})
		require.NoError(t, err)
		require.Equal(t, 7, got)
		require.Equal(t, 2, calls, "should succeed on the first retry")
	})

	t.Run("non_retryable_error", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("boom")
		var calls int
		_, err := loadWithRetry(context.Background(), func() (int, error) {
			calls++
			return 0, wantErr
		})
		require.ErrorIs(t, err, wantErr)
		require.Equal(t, 1, calls, "a non-retryable error must not be retried")
	})
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
