package rqlite_test

import (
	"fmt"
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

func TestAlterTableRename(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	uniq := stringz.Uniq8()
	oldName := "rename_old_" + uniq
	newName := "rename_new_" + uniq
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: oldName}, true)
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: newName}, true)
	})

	tblDef := schema.NewTable(oldName, []string{"id"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableRename(th.Context, db, oldName, newName))

	exists, err := drvr.TableExists(th.Context, db, newName)
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = drvr.TableExists(th.Context, db, oldName)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestAlterTableAddColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "addcol_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id"}, []kind.Kind{kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, "name", kind.Text))
	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, "ts", kind.Datetime))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Len(t, md.Columns, 3)
	require.Equal(t, "name", md.Columns[1].Name)
	require.Equal(t, kind.Text, md.Columns[1].Kind)
	require.Equal(t, "ts", md.Columns[2].Name)
	require.Equal(t, kind.Datetime, md.Columns[2].Kind)
}

func TestAlterTableRenameColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "renamecol_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "first_name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, tblName, "first_name", "given_name"))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	colNames := make([]string, len(md.Columns))
	for i, c := range md.Columns {
		colNames[i] = c.Name
	}
	require.Equal(t, []string{"id", "given_name"}, colNames)
}

func TestTruncate_NoReset(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "trunc_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	for i := 1; i <= 3; i++ {
		_, err = db.ExecContext(th.Context,
			fmt.Sprintf(`INSERT INTO %q (id, name) VALUES (?, ?)`, tblName), i, "x")
		require.NoError(t, err)
	}

	affected, err := drvr.Truncate(th.Context, src, tblName, false)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	var count int64
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT COUNT(*) FROM %q`, tblName)).Scan(&count))
	require.Equal(t, int64(0), count)
}

func TestTruncate_Reset(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "trunc_reset_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	tblDef.PKColName = "id"
	tblDef.AutoIncrement = true
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	// Insert 3 rows so sqlite_sequence has data for this table.
	for i := 0; i < 3; i++ {
		_, err = db.ExecContext(th.Context,
			fmt.Sprintf(`INSERT INTO %q (name) VALUES (?)`, tblName), "x")
		require.NoError(t, err)
	}

	affected, err := drvr.Truncate(th.Context, src, tblName, true)
	require.NoError(t, err)
	require.Equal(t, int64(3), affected)

	// Insert again; the new id should be 1, not 4.
	res, err := db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (name) VALUES (?)`, tblName), "y")
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	require.Equal(t, int64(1), id, "AUTOINCREMENT counter should have been reset")
}

func TestCopyTable_StructureOnly(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "actor_copy_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	affected, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblActor}, tablefq.T{Table: dstName}, false)
	require.NoError(t, err)
	require.Equal(t, int64(0), affected)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.Equal(t, dstName, md.Name)
	require.Equal(t, int64(0), md.RowCount)

	src2 := th.Source(sakila.Rq)
	srcMd, err := th.Open(src2).TableMetadata(th.Context, sakila.TblActor)
	require.NoError(t, err)
	require.Len(t, md.Columns, len(srcMd.Columns))
}

func TestCopyTable_WithData(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	dstName := "actor_data_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: dstName}, true)
	})

	affected, err := drvr.CopyTable(th.Context, db,
		tablefq.T{Table: sakila.TblActor}, tablefq.T{Table: dstName}, true)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), affected)

	md, err := grip.TableMetadata(th.Context, dstName)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), md.RowCount)
}

func TestAlterTableColumnKinds(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Rq)
	grip := th.Open(src)
	drvr := grip.SQLDriver()
	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	tblName := "kinds_" + stringz.Uniq8()
	t.Cleanup(func() {
		_ = drvr.DropTable(th.Context, db, tablefq.T{Table: tblName}, true)
	})

	tblDef := schema.NewTable(tblName, []string{"a", "b"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))

	_, err = db.ExecContext(th.Context,
		fmt.Sprintf(`INSERT INTO %q (a, b) VALUES (?, ?)`, tblName), 42, "hello")
	require.NoError(t, err)

	// Swap kinds: a INTEGER -> TEXT, b TEXT -> INTEGER.
	require.NoError(t, drvr.AlterTableColumnKinds(th.Context, db, tblName,
		[]string{"a", "b"}, []kind.Kind{kind.Text, kind.Int}))

	md, err := grip.TableMetadata(th.Context, tblName)
	require.NoError(t, err)
	require.Equal(t, tblName, md.Name)
	require.Equal(t, kind.Text, md.Columns[0].Kind)
	require.Equal(t, kind.Int, md.Columns[1].Kind)

	// Row data should round-trip; sqlite is permissive about typing.
	var gotA, gotB string
	require.NoError(t, db.QueryRowContext(th.Context,
		fmt.Sprintf(`SELECT a, b FROM %q`, tblName)).Scan(&gotA, &gotB))
	require.Equal(t, "42", gotA)
	require.Equal(t, "hello", gotB)
}
