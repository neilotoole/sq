package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdTblCopy(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			srcTblHandle := src.Handle + "." + sakila.TblActor
			destTbl1 := stringz.UniqTableName(sakila.TblActor)

			// Register the drop before executing the copy: if the command or
			// an assertion below fails, the table must still be reaped from
			// the shared container. Helper.DropTable uses ifExists semantics,
			// so it's a no-op if the table was never created.
			t.Cleanup(func() { th.DropTable(src, tablefq.From(destTbl1)) })
			tr1 := testrun.New(th.Context, t, nil).Add(*src)
			err := tr1.Exec("tbl", "copy", "--data=false", srcTblHandle, src.Handle+"."+destTbl1)
			require.NoError(t, err)
			require.Equal(t, int64(0), th.RowCount(src, destTbl1),
				"should not have copied any rows because --data=false")

			// --data=true
			destTbl2 := stringz.UniqTableName(sakila.TblActor)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(destTbl2)) })
			tr2 := testrun.New(th.Context, t, nil).Add(*src)
			err = tr2.Exec("tbl", "copy", "--data=true", srcTblHandle, src.Handle+"."+destTbl2)
			require.NoError(t, err)
			require.Equal(t, int64(sakila.TblActorCount), th.RowCount(src, destTbl2),
				"should have copied rows because --data=true")
		})
	}
}

func TestCmdTblDrop(t *testing.T) { //nolint:tparallel
	for _, handle := range sakila.SQLAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			// dropAfter=true registers an ifExists drop at creation time, so
			// the table is reaped even if an assertion below fails; it's a
			// no-op after the "tbl drop" command drops the table.
			destTblName := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

			tblMeta, err := th.Open(src).TableMetadata(th.Context, destTblName)
			require.NoError(t, err) // verify that the table exists
			// Oracle returns identifiers in their stored case (upper for
			// unquoted), so compare table names case-insensitively.
			if src.Type == drivertype.Oracle {
				require.True(t, strings.EqualFold(destTblName, tblMeta.Name),
					"table name got %q want ~%q", tblMeta.Name, destTblName)
			} else {
				require.Equal(t, destTblName, tblMeta.Name)
			}
			require.Equal(t, int64(sakila.TblActorCount), tblMeta.RowCount)

			err = testrun.New(th.Context, t, nil).Add(*src).Exec("tbl", "drop", src.Handle+"."+destTblName)
			require.NoError(t, err)

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
