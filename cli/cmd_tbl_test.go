package cli_test

import (
	"testing"

	"github.com/neilotoole/sq/libsq/core/tablefq"

	"github.com/neilotoole/sq/cli/testrun"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdTblCopy(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			srcTblHandle := src.Handle + "." + sakila.TblActor
			destTbl1 := stringz.UniqTableName(sakila.TblActor)

			tr1 := testrun.New(th.Context, t, nil).Add(*src)
			err := tr1.Exec("tbl", "copy", "--data=false", srcTblHandle, src.Handle+"."+destTbl1)
			require.NoError(t, err)
			defer th.DropTable(src, tablefq.From(destTbl1))
			require.Equal(t, int64(0), th.RowCount(src, destTbl1),
				"should not have copied any rows because --data=false")

			// --data=true
			tr2 := testrun.New(th.Context, t, nil).Add(*src)
			destTbl2 := stringz.UniqTableName(sakila.TblActor)
			err = tr2.Exec("tbl", "copy", "--data=true", srcTblHandle, src.Handle+"."+destTbl2)
			require.NoError(t, err)
			defer th.DropTable(src, tablefq.From(destTbl2))
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, destTbl2),
				"should have copied rows because --data=true")
		})
	}
}

func TestCmdTblDrop(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			destTblName := th.CopyTable(false, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)
			needsDrop := true

			defer func() {
				if needsDrop {
					th.DropTable(src, tablefq.From(destTblName))
				}
			}()

			tblMeta, err := th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err) // verify that the table exists
			require.Equal(t, destTblName, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)

			err = testrun.New(th.Context, t, nil).Add(*src).Exec("tbl", "drop", src.Handle+"."+destTblName)
			require.NoError(t, err)
			needsDrop = false

			tblMeta, err = th.Open(src).TableMetadata(th.Context, destTblName)
			require.Error(t, err, "should get an error because the table was dropped")
			require.Nil(t, tblMeta)
		})
	}
}

func TestCmdTblTruncate(t *testing.T) {
	t.Parallel()

	testCases := []string{sakila.MS, sakila.My, sakila.Pg9}
	for _, handle := range testCases {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			destTblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			tblMeta, err := th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err) // verify that the table exists
			require.Equal(t, destTblName, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)

			err = testrun.New(th.Context, t, nil).Add(*src).Exec("tbl", "truncate", src.Handle+"."+destTblName)
			require.NoError(t, err)
			tblMeta, err = th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err)
			require.Equal(t, int64(0), tblMeta.RowCount)
		})
	}
}
