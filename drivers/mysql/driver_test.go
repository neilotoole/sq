package mysql_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriver_DBProperties(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.My)
	props, err := drvr.DBProperties(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, props)
}

func TestDriver_SchemaAndCatalog(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	schma, err := drvr.CurrentSchema(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "sakila", schma)

	exists, err := drvr.SchemaExists(ctx, db, "sakila")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = drvr.SchemaExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, exists, "empty schema name is never reported as existing")

	exists, err = drvr.SchemaExists(ctx, db, "no_such_schema_"+stringz.Uniq8())
	require.NoError(t, err)
	require.False(t, exists)

	schemas, err := drvr.ListSchemas(ctx, db)
	require.NoError(t, err)
	require.Contains(t, schemas, "sakila")

	schemaMetas, err := drvr.ListSchemaMetadata(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, schemaMetas)

	// MySQL doesn't really support catalogs; sq surfaces the fixed "def".
	cat, err := drvr.CurrentCatalog(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "def", cat)

	cats, err := drvr.ListCatalogs(ctx, db)
	require.NoError(t, err)
	require.Equal(t, []string{"def"}, cats)

	ok, err := drvr.CatalogExists(ctx, db, "def")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = drvr.CatalogExists(ctx, db, "other")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestDriver_ListTableNames(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	tablesOnly, err := drvr.ListTableNames(ctx, db, "", true, false)
	require.NoError(t, err)
	require.Contains(t, tablesOnly, sakila.TblActor)

	viewsOnly, err := drvr.ListTableNames(ctx, db, "", false, true)
	require.NoError(t, err)
	require.NotContains(t, viewsOnly, sakila.TblActor, "actor is a base table, not a view")

	both, err := drvr.ListTableNames(ctx, db, "sakila", true, true)
	require.NoError(t, err)
	require.Contains(t, both, sakila.TblActor)

	// Neither tables nor views: the contract is an empty slice.
	neither, err := drvr.ListTableNames(ctx, db, "", false, false)
	require.NoError(t, err)
	require.Empty(t, neither)
}

func TestDriver_TableExistsAndColumnTypes(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	exists, err := drvr.TableExists(ctx, db, sakila.TblActor)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = drvr.TableExists(ctx, db, "no_such_table_"+stringz.Uniq8())
	require.NoError(t, err)
	require.False(t, exists)

	colTypes, err := drvr.TableColumnTypes(ctx, db, sakila.TblActor, nil)
	require.NoError(t, err)
	require.NotEmpty(t, colTypes)

	// Restrict to a column subset.
	colTypes, err = drvr.TableColumnTypes(ctx, db, sakila.TblActor, []string{"actor_id", "first_name"})
	require.NoError(t, err)
	require.Len(t, colTypes, 2)
}

func TestDriver_CreateAlterDropTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	tblName := stringz.UniqTableName("review_alter")
	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	tblDef.PKColName = "id"
	tblDef.AutoIncrement = true

	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	exists, err := drvr.TableExists(ctx, db, tblName)
	require.NoError(t, err)
	require.True(t, exists)

	// Add a column.
	require.NoError(t, drvr.AlterTableAddColumn(ctx, db, tblName, "age", kind.Int))

	// Rename the new column.
	require.NoError(t, drvr.AlterTableRenameColumn(ctx, db, tblName, "age", "years"))

	colTypes, err := drvr.TableColumnTypes(ctx, db, tblName, nil)
	require.NoError(t, err)
	require.Len(t, colTypes, 3)

	// AlterTableColumnKinds is not implemented for MySQL.
	err = drvr.AlterTableColumnKinds(ctx, db, tblName, []string{"years"}, []kind.Kind{kind.Text})
	require.Error(t, err)

	// Rename the table.
	newName := stringz.UniqTableName("review_alter_renamed")
	require.NoError(t, drvr.AlterTableRename(ctx, db, tblName, newName))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(newName)) })

	exists, err = drvr.TableExists(ctx, db, newName)
	require.NoError(t, err)
	require.True(t, exists)

	// DropTable with ifExists on a missing table is a no-op.
	require.NoError(t, drvr.DropTable(ctx, db, tablefq.From("no_such_"+stringz.Uniq8()), true))
}

func TestDriver_CopyTableAndTruncate(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	// Copy with data.
	dst := tablefq.From(stringz.UniqTableName("review_copy"))
	affected, err := drvr.CopyTable(ctx, db, tablefq.From(sakila.TblActor), dst, true)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), affected)
	t.Cleanup(func() { th.DropTable(src, dst) })

	// Truncate returns the count of rows removed.
	truncated, err := drvr.Truncate(ctx, src, dst.Table, true)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), truncated)

	// Copy without data (schema only).
	dstEmpty := tablefq.From(stringz.UniqTableName("review_copy_empty"))
	affected, err = drvr.CopyTable(ctx, db, tablefq.From(sakila.TblActor), dstEmpty, false)
	require.NoError(t, err)
	require.Zero(t, affected)
	t.Cleanup(func() { th.DropTable(src, dstEmpty) })
}

func TestDriver_PrepareInsertAndUpdateStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	tblName := stringz.UniqTableName("review_prep")
	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Insert via prepared statement.
	insErr := func() error {
		execer, err := drvr.PrepareInsertStmt(ctx, db, tblName, []string{"id", "name"}, 1)
		if err != nil {
			return err
		}
		defer func() { require.NoError(t, execer.Close()) }()

		rec := []any{int64(1), "alice"}
		require.NoError(t, execer.Munge(rec))
		affected, err := execer.Exec(ctx, rec...)
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)
		return nil
	}()
	require.NoError(t, insErr)

	// Update via prepared statement.
	upErr := func() error {
		execer, err := drvr.PrepareUpdateStmt(ctx, db, tblName, []string{"name"}, "id = 1")
		if err != nil {
			return err
		}
		defer func() { require.NoError(t, execer.Close()) }()

		rec := []any{"bob"}
		require.NoError(t, execer.Munge(rec))
		affected, err := execer.Exec(ctx, "bob")
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)
		return nil
	}()
	require.NoError(t, upErr)

	sink, err := th.QuerySQL(src, nil, "SELECT name FROM "+tblName+" WHERE id = 1")
	require.NoError(t, err)
	require.Len(t, sink.Recs, 1)
}

func TestDriver_NewBatchInsert(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context

	tblName := stringz.UniqTableName("review_batch")
	tblDef := schema.NewTable(tblName, []string{"id", "name"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	// Batch insert requires a single dedicated connection, not the pool.
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	bi, err := drvr.NewBatchInsert(ctx, "insert", conn, src, tblName, []string{"id", "name"})
	require.NoError(t, err)

	recs := [][]any{{int64(1), "alice"}, {int64(2), "bob"}}
	for _, rec := range recs {
		require.NoError(t, bi.Munge(rec))
		select {
		case err = <-bi.ErrCh:
			close(bi.RecordCh)
			t.Fatal(err)
		case bi.RecordCh <- rec:
		}
	}
	close(bi.RecordCh)

	require.NoError(t, <-bi.ErrCh)
	require.Equal(t, int64(2), bi.Written())
}

func TestDriver_Ping(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.My)
	require.NoError(t, drvr.Ping(th.Context, src, driver.ModeReadWrite))
}

// TestDriver_OpenWithSchema exercises the doOpen branch that overrides the
// connection's default schema from src.Schema.
func TestDriver_OpenWithSchema(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.My)

	// Clone the source and pin an explicit default schema.
	withSchema := src.Clone()
	withSchema.Schema = "sakila"

	grip, err := drvr.Open(th.Context, withSchema, driver.ModeReadWrite)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, grip.Close()) })

	db, err := grip.DB(th.Context)
	require.NoError(t, err)

	var schma string
	require.NoError(t, db.QueryRowContext(th.Context, "SELECT DATABASE()").Scan(&schma))
	require.Equal(t, "sakila", schma)
}

// TestDriver_ClosedDBErrors closes the DB out from under the driver and
// asserts every read/metadata method surfaces the error rather than a
// zero value, exercising the errw error-return branches.
func TestDriver_ClosedDBErrors(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.My)
	ctx := th.Context
	require.NoError(t, db.Close())

	_, err := drvr.CurrentSchema(ctx, db)
	require.Error(t, err)
	_, err = drvr.ListSchemas(ctx, db)
	require.Error(t, err)
	_, err = drvr.SchemaExists(ctx, db, "sakila")
	require.Error(t, err)
	_, err = drvr.ListSchemaMetadata(ctx, db)
	require.Error(t, err)
	_, err = drvr.CurrentCatalog(ctx, db)
	require.Error(t, err)
	_, err = drvr.ListCatalogs(ctx, db)
	require.Error(t, err)
	_, err = drvr.ListTableNames(ctx, db, "", true, true)
	require.Error(t, err)
	_, err = drvr.TableExists(ctx, db, sakila.TblActor)
	require.Error(t, err)
	_, err = drvr.TableColumnTypes(ctx, db, sakila.TblActor, nil)
	require.Error(t, err)
	_, err = drvr.DBProperties(ctx, db)
	require.Error(t, err)

	// Methods that first resolve dest column metadata fail at that step.
	_, err = drvr.PrepareInsertStmt(ctx, db, sakila.TblActor, []string{"actor_id"}, 1)
	require.Error(t, err)
	_, err = drvr.PrepareUpdateStmt(ctx, db, sakila.TblActor, []string{"actor_id"}, "")
	require.Error(t, err)

	require.Error(t, drvr.AlterTableAddColumn(ctx, db, sakila.TblActor, "x", kind.Int))
	require.Error(t, drvr.AlterTableRename(ctx, db, sakila.TblActor, "x"))
	require.Error(t, drvr.AlterTableRenameColumn(ctx, db, sakila.TblActor, "first_name", "x"))
	require.Error(t, drvr.DropTable(ctx, db, tablefq.From(sakila.TblActor), false))
	_, err = drvr.CopyTable(ctx, db, tablefq.From(sakila.TblActor), tablefq.From("x"), true)
	require.Error(t, err)
}

// TestDriver_BadSource exercises the doOpen failure path: a source whose
// location can't be parsed into a DSN must fail Open, Ping and Truncate.
func TestDriver_BadSource(t *testing.T) {
	t.Parallel()

	th := testh.New(t)
	drvr := th.SQLDriverFor(&source.Source{Type: drivertype.MySQL, Location: "mysql://x"})

	badSrc := &source.Source{
		Handle:   "@bad",
		Type:     drivertype.MySQL,
		Location: "mysql://", // too short to yield a DSN
	}

	_, err := drvr.Open(th.Context, badSrc, driver.ModeReadWrite)
	require.Error(t, err)

	err = drvr.Ping(th.Context, badSrc, driver.ModeReadWrite)
	require.Error(t, err)

	_, err = drvr.Truncate(th.Context, badSrc, "actor", true)
	require.Error(t, err)
}

// TestDriver_RenderFuncs drives the MySQL-specific function renderers
// (LIKE BINARY family, ilike fallthrough, rownum, catalog) end-to-end
// through SLQ so the dialect overrides in Renderer() are exercised.
func TestDriver_RenderFuncs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	h := sakila.My

	testCases := []struct {
		name string
		slq  string
	}{
		{"contains", h + ` | .actor | where(contains(.first_name, "AN"))`},
		{"startswith", h + ` | .actor | where(startswith(.first_name, "PEN"))`},
		{"endswith", h + ` | .actor | where(endswith(.last_name, "MAN"))`},
		{"like", h + ` | .actor | where(like(.first_name, "PEN%"))`},
		{"icontains", h + ` | .actor | where(icontains(.first_name, "an"))`},
		{"rownum", h + ` | .actor | rownum(), .actor_id | order_by(.actor_id)`},
		{"catalog", h + ` | catalog()`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Each subtest gets its own helper so a sibling's grip
			// teardown can't close a shared *sql.DB mid-query.
			th := testh.New(t)
			sink, err := th.QuerySLQ(tc.slq, nil)
			require.NoError(t, err)
			require.NotNil(t, sink)
		})
	}
}
