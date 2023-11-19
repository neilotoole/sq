//go:build sqlite_vtable && sqlite_fts5

package sqlite3_test

import (
	"testing"

	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tutil"
	"github.com/stretchr/testify/require"
)

func TestExtension_fts5(t *testing.T) {
	const tblActorFts = "actor_fts"

	th := testh.New(t)
	src := th.Add(&source.Source{
		Handle:   "@fts",
		Type:     sqlite3.Type,
		Location: "sqlite3://" + tutil.MustAbsFilepath("testdata", "sakila_fts5.db"),
	})

	srcMeta, err := th.SourceMetadata(src)
	require.NoError(t, err)
	require.Equal(t, src.Handle, srcMeta.Handle)
	tblMeta1 := srcMeta.Table(tblActorFts)
	require.NotNil(t, tblMeta1)
	require.Equal(t, tblActorFts, tblMeta1.Name)
	require.Equal(t, sqlz.TableTypeVirtual, tblMeta1.TableType)

	require.Equal(t, "actor_id", tblMeta1.Columns[0].Name)
	require.Equal(t, "integer", tblMeta1.Columns[0].ColumnType)
	require.Equal(t, "integer", tblMeta1.Columns[0].BaseType)
	require.Equal(t, kind.Int, tblMeta1.Columns[0].Kind)
	require.Equal(t, "first_name", tblMeta1.Columns[1].Name)
	require.Equal(t, "text", tblMeta1.Columns[1].ColumnType)
	require.Equal(t, "text", tblMeta1.Columns[1].BaseType)
	require.Equal(t, kind.Text, tblMeta1.Columns[1].Kind)
	require.Equal(t, "last_name", tblMeta1.Columns[2].Name)
	require.Equal(t, "text", tblMeta1.Columns[2].ColumnType)
	require.Equal(t, "text", tblMeta1.Columns[2].BaseType)
	require.Equal(t, kind.Text, tblMeta1.Columns[2].Kind)
	require.Equal(t, "last_update", tblMeta1.Columns[3].Name)
	require.Equal(t, "text", tblMeta1.Columns[3].ColumnType)
	require.Equal(t, "text", tblMeta1.Columns[3].BaseType)
	require.Equal(t, kind.Text, tblMeta1.Columns[3].Kind)

	tblMeta2, err := th.TableMetadata(src, tblActorFts)
	require.NoError(t, err)
	require.Equal(t, tblActorFts, tblMeta2.Name)
	require.Equal(t, sqlz.TableTypeVirtual, tblMeta2.TableType)
	require.EqualValues(t, *tblMeta1, *tblMeta2)

	// Verify that the (non-virtual) "actor" table has its type set correctly.
	actorMeta1 := srcMeta.Table(sakila.TblActor)
	actorMeta2, err := th.TableMetadata(src, sakila.TblActor)
	require.NoError(t, err)
	require.Equal(t, actorMeta1.TableType, sqlz.TableTypeTable)
	require.Equal(t, *actorMeta1, *actorMeta2)
}
