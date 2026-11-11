package driver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/vector"
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

func TestDefaultInsertMungeFunc_Vector(t *testing.T) {
	meta := record.Meta{record.NewFieldMeta(&record.ColumnTypeData{
		Name: "v", Kind: kind.Vector, Nullable: true, HasNullable: true,
	}, "v")}
	rec := record.Record{vector.Vector{Elem: vector.F32, Raw: []byte{1, 2}}}
	require.NoError(t, driver.DefaultInsertMungeFunc("t", meta)(rec))
	require.Equal(t, []byte{1, 2}, rec[0])
}
