package mysql_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/fixt"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestSmoke(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.MyAll() {
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

func TestDriver_CreateTable_NotNullDefault(t *testing.T) {
	t.Parallel()

	testCases := sakila.MyAll()
	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th, src, drvr, _, db := testh.NewWith(t, handle)

			tblName := stringz.UniqTableName(t.Name())
			colNames, colKinds := fixt.ColNamePerKind(drvr.Dialect().IntBool, false, false)

			tblDef := sqlmodel.NewTableDef(tblName, colNames, colKinds)
			for _, colDef := range tblDef.Cols {
				colDef.NotNull = true
				colDef.HasDefault = true
			}

			err := drvr.CreateTable(th.Context, db, tblDef)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tblName) })

			// MySQL doesn't support default values for TEXT or BLOB
			// See: https://bugs.mysql.com/bug.php?id=21532
			// So, instead of "INSERT INTO tblName () VALUES ()" we
			// need to provide explicit values for col_text and col_bytes.
			insertDefaultStmt := "INSERT INTO " + tblName + " (col_text, col_bytes) VALUES (?, ?)"
			affected := th.ExecSQL(src, insertDefaultStmt, "", []byte{})
			require.Equal(t, int64(1), affected)

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
		})
	}
}

// TestBug252_ShowCollation_uint64 tests the
// bug https://github.com/neilotoole/sq/issues/252.
func TestBug252_ShowCollation_uint64(t *testing.T) {
	testCases := sakila.MyAll()
	for _, handle := range testCases {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			th, src, _, _, _ := testh.NewWith(t, handle)

			sink, err := th.QuerySQL(src, "SHOW COLLATION")
			require.NoError(t, err)
			require.NotNil(t, sink)
		})
	}
}
