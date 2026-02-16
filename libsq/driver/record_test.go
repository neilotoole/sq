package driver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
)

// TestStmtExecer_Close_NilStmt verifies that StmtExecer.Close is
// nil-safe when constructed with a nil *sql.Stmt. This supports
// drivers (e.g. ClickHouse) that bypass PrepareContext() and pass
// nil for stmt to NewStmtExecer.
func TestStmtExecer_Close_NilStmt(t *testing.T) {
	execer := driver.NewStmtExecer(nil, nil, nil, nil)
	err := execer.Close()
	require.NoError(t, err)
}
