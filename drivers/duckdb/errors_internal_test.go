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

// TestErrw_NotExist verifies that a DuckDB "table does not exist" error is
// mapped to driver.NotExistError by errw.
func TestErrw_NotExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	rawDB, err := sql.Open(dbDrvr, dbPath)
	require.NoError(t, err)
	defer rawDB.Close()

	_, err = rawDB.ExecContext(context.Background(), "SELECT * FROM nonexistent_table_xyz")
	require.Error(t, err)

	wrapped := errw(err)
	require.True(t, errz.Has[*driver.NotExistError](wrapped),
		"expected driver.NotExistError, got: %v (%T)", wrapped, wrapped)
}

// TestErrw_Nil verifies that errw(nil) returns nil.
func TestErrw_Nil(t *testing.T) {
	require.NoError(t, errw(nil))
}
