package duckdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// TestErrw_NotExist verifies that DuckDB "does not exist" catalog errors for
// each kind of object (table, view, schema) are mapped to driver.NotExistError
// by errw.
func TestErrw_NotExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	rawDB, err := sql.Open(dbDrvr, dbPath)
	require.NoError(t, err)
	defer rawDB.Close()

	ctx := context.Background()

	testCases := []struct {
		name string
		sql  string
	}{
		{"table", "SELECT * FROM nonexistent_table_xyz"},
		{"view", "DROP VIEW nonexistent_view_xyz"},
		{"schema", "DROP SCHEMA nonexistent_schema_xyz"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rawDB.ExecContext(ctx, tc.sql)
			require.Error(t, err)

			wrapped := errw(err)
			require.True(t, errz.Has[*driver.NotExistError](wrapped),
				"expected driver.NotExistError, got: %v (%T)", wrapped, wrapped)
		})
	}
}

// TestErrw_Nil verifies that errw(nil) returns nil.
func TestErrw_Nil(t *testing.T) {
	require.NoError(t, errw(nil))
}

// TestErrw_PassThrough verifies that an error not matching any special-case
// pattern (e.g. a syntax error) is wrapped but not classified as NotExistError.
func TestErrw_PassThrough(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	rawDB, err := sql.Open(dbDrvr, dbPath)
	require.NoError(t, err)
	defer rawDB.Close()

	_, err = rawDB.ExecContext(context.Background(), "SELEKT 1")
	require.Error(t, err)

	wrapped := errw(err)
	require.False(t, errz.Has[*driver.NotExistError](wrapped),
		"syntax error must not be classified as NotExistError; got: %v", wrapped)
}
