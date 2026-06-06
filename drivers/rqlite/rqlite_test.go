package rqlite_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestSmoke exercises Open/Ping plus a basic SELECT against the
// sakiladb/rqlite container. The test is skipped under `go test -short`
// or when SQ_TEST_SRC__SAKILA_RQ is unset (the standard pattern for
// network-backed sakila sources).
func TestSmoke(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	require.Equal(t, drivertype.Rqlite, src.Type)

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+sakila.TblActor)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActorCount, len(sink.Recs))
}

// TestSourceMetadata verifies that getSourceMetadata returns the
// expected shape: rqlite driver, "main" schema, and the right
// table/view counts (16 tables, 5 views in the bundled Sakila).
func TestSourceMetadata(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)

	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err)
	require.Equal(t, drivertype.Rqlite, md.Driver)
	require.Equal(t, "main", md.Schema)
	require.Equal(t, "default", md.Catalog)
	require.NotEmpty(t, md.DBVersion, "expected SQLite version from rqlite")
	require.Equal(t, int64(16), md.TableCount)
	require.Equal(t, int64(5), md.ViewCount)
}

// TestTableMetadata_Actor verifies the per-table metadata path:
// column kinds, primary-key flag, and row count for the actor table.
func TestTableMetadata_Actor(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)

	tbl, err := grip.TableMetadata(th.Context, sakila.TblActor)
	require.NoError(t, err)
	require.Equal(t, sakila.TblActor, tbl.Name)
	require.Equal(t, int64(sakila.TblActorCount), tbl.RowCount)

	gotKinds := make([]kind.Kind, len(tbl.Columns))
	for i, col := range tbl.Columns {
		gotKinds[i] = col.Kind
	}
	// actor: actor_id (decimal due to NUMERIC affinity), first_name,
	// last_name (text), last_update (datetime). sakila.TblActorColKinds
	// returns kind.Int for actor_id; the SQLite-on-rqlite shape uses
	// NUMERIC → decimal, so we assert the column kinds explicitly here
	// rather than reusing the shared helper.
	require.Equal(t, []kind.Kind{kind.Decimal, kind.Text, kind.Text, kind.Datetime}, gotKinds)
	require.True(t, tbl.Columns[0].PrimaryKey, "actor_id should be primary key")
}

func TestCreateTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "actor_w_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName,
		[]string{"id", "name", "ts"},
		[]kind.Kind{kind.Int, kind.Text, kind.Datetime},
	)
	tblDef.PKColName = "id"

	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	got, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Equal(t, tblName, got.Name)
	require.Len(t, got.Columns, 3)
	require.Equal(t, kind.Int, got.Columns[0].Kind)
	require.Equal(t, kind.Text, got.Columns[1].Kind)
	require.Equal(t, kind.Datetime, got.Columns[2].Kind)
}
