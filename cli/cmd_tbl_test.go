package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/stringz"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdTblCopy(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			srcTblHandle := src.Handle + "." + sakila.TblActor
			destTblName := stringz.UniqTableName(sakila.TblActor)

			ru := newRun(t).add(*src)
			err := ru.exec("tbl", "copy", "--data=false", srcTblHandle, src.Handle+"."+destTblName)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, destTblName) })
			require.Equal(t, int64(0), th.RowCount(src, destTblName), "should not have copied any rows because --data=false")

			destTblName = stringz.UniqTableName(sakila.TblActor)
			err = ru.exec("tbl", "copy", "--data=true", srcTblHandle, src.Handle+"."+destTblName)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, destTblName) })
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, destTblName), "should have copied rows because --data=true")
		})
	}
}

func TestCmdTblDrop(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			destTblName := th.CopyTable(false, src, sakila.TblActor, "", true)
			t.Cleanup(func() { th.DropTable(src, destTblName) })

			tblMeta, err := th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err) // verify that the table exists
			require.Equal(t, destTblName, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)

			err = newRun(t).add(*src).exec("tbl", "drop", src.Handle+"."+destTblName)
			require.NoError(t, err)
			tblMeta, err = th.Open(src).TableMetadata(th.Context, destTblName)
			require.Error(t, err, "should get an error because the table was dropped")
			require.Nil(t, tblMeta)
		})
	}
}

func TestCmdTblTruncate(t *testing.T) {
	t.Parallel()

	for _, handle := range sakila.SQLAll() {
		handle := handle

		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			destTblName := th.CopyTable(false, src, sakila.TblActor, "", true)
			t.Cleanup(func() { th.DropTable(src, destTblName) })

			tblMeta, err := th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err) // verify that the table exists
			require.Equal(t, destTblName, tblMeta.Name)
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)

			err = newRun(t).add(*src).exec("tbl", "truncate", src.Handle+"."+destTblName)
			require.NoError(t, err)
			tblMeta, err = th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err)
			require.Equal(t, int64(0), tblMeta.RowCount)
		})
	}
}
