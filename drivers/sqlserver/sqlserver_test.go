package sqlserver_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, _, _, _ := testh.NewWith(t, handle)
			sink, err := th.QuerySQL(src, "SELECT * FROM actor")
			require.NoError(t, err)
			require.Equal(t, len(sakila.TblActorCols()), len(sink.RecMeta))
			require.Equal(t, sakila.TblActorCount, len(sink.Recs))
		})
	}
}

func TestDriverBehavior(t *testing.T) {
	t.Parallel()

	// This test exists to help understand the behavior of the driver impl.
	// It can be deleted eventually.

	th := testh.New(t)
	src := th.Source(sakila.MS)
	db, _ := th.Open(src).DB()

	const query = "SELECT * FROM payment ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

	rows, err := db.QueryContext(th.Context, query)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, rows.Close()) })

	colTypes, err := rows.ColumnTypes()
	require.NoError(t, err)

	for i, colType := range colTypes {
		nullable, ok := colType.Nullable()
		t.Logf("%d:	%s	%s	%s	nullable,ok={%v,%v}", i, colType.Name(), colType.DatabaseTypeName(),
			colType.ScanType().Name(), nullable, ok)

		if !nullable {
			scanType := colType.ScanType()
			z := reflect.Zero(scanType)
			t.Logf("zero: %T %v", z, z)
		}
	}
}

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.MS}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, dbase, _ := testh.NewWith(t, handle)
			db, err := dbase.DB()
			require.NoError(t, err)

			tblName := stringz.UniqTableName(t.Name())
			colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)

			tblDef := sqlmodel.NewTableDef(tblName, colNames, colKinds)
			for _, colDef := range tblDef.Cols {
				colDef.NotNull = true
				colDef.HasDefault = true
			}

			err = drvr.CreateTable(th.Context, db, tblDef)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tblName) })

			th.InsertDefaultRow(src, tblName)

			sink, err := th.QuerySQL(src, "SELECT * FROM "+tblName)
			require.NoError(t, err)
			require.Equal(t, 1, len(sink.Recs))
			require.Equal(t, len(colNames), len(sink.RecMeta))
			for i := range colNames {
				require.NotNil(t, sink.Recs[0][i])
				nullable, ok := sink.RecMeta[i].Nullable()
				require.True(t, ok)
				require.False(t, nullable)
			}

			// Check kind.Bytes is handled correctly
			const iBytes = 8 // the index of col_bytes
			require.Equal(t, kind.Bytes, colKinds[iBytes])
			b, ok := sink.Recs[0][iBytes].([]byte)
			require.True(t, ok)
			require.NotNil(t, b)
			require.Equal(t, 0, len(b), "b should be non-nil but zero length")
		})
	}
}
