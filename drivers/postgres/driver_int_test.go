package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func TestDriver_Ping_Integration(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.Pg)
	require.NoError(t, drvr.Ping(th.Context, src, driver.ModeReadWrite))
}

func TestDriver_DBProperties(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	props, err := drvr.DBProperties(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, props)
	require.Contains(t, props, "server_version")
}

func TestDriver_Truncate(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)

	tblName := stringz.UniqTableName("truncate_test")
	tblDef := schema.NewTable(tblName, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	const wantRows = 3
	for i := range wantRows {
		_, err := db.ExecContext(th.Context,
			`INSERT INTO "`+tblName+`" (name, val) VALUES ($1, $2)`, "n", i)
		require.NoError(t, err)
	}

	affected, err := drvr.Truncate(th.Context, src, tblName, true)
	require.NoError(t, err)
	require.Equal(t, int64(wantRows), affected)

	var count int64
	require.NoError(t, db.QueryRowContext(th.Context, `SELECT COUNT(*) FROM "`+tblName+`"`).Scan(&count))
	require.Zero(t, count)
}

func TestDriver_Schemas(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	cur, err := drvr.CurrentSchema(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "public", cur)

	// SchemaExists with empty arg short-circuits to false.
	exists, err := drvr.SchemaExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = drvr.SchemaExists(ctx, db, "definitely_not_a_schema_xyz")
	require.NoError(t, err)
	require.False(t, exists)

	schemaName := stringz.UniqSuffix("test_schema")
	require.NoError(t, drvr.CreateSchema(ctx, db, schemaName))
	t.Cleanup(func() { assert.NoError(t, drvr.DropSchema(ctx, db, schemaName)) })

	exists, err = drvr.SchemaExists(ctx, db, schemaName)
	require.NoError(t, err)
	require.True(t, exists)

	schemas, err := drvr.ListSchemas(ctx, db)
	require.NoError(t, err)
	require.Contains(t, schemas, schemaName)
	require.Contains(t, schemas, "public")

	schemaMetas, err := drvr.ListSchemaMetadata(ctx, db)
	require.NoError(t, err)
	var found bool
	for _, sm := range schemaMetas {
		if sm.Name == schemaName {
			found = true
			require.Equal(t, "sakila", sm.Catalog)
		}
	}
	require.True(t, found, "new schema should appear in ListSchemaMetadata")
}

func TestDriver_Catalogs(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	cur, err := drvr.CurrentCatalog(ctx, db)
	require.NoError(t, err)
	require.Equal(t, "sakila", cur)

	catalogs, err := drvr.ListCatalogs(ctx, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalogs)
	require.Equal(t, "sakila", catalogs[0], "current catalog must be first")

	exists, err := drvr.CatalogExists(ctx, db, "sakila")
	require.NoError(t, err)
	require.True(t, exists)

	// Empty arg short-circuits to false.
	exists, err = drvr.CatalogExists(ctx, db, "")
	require.NoError(t, err)
	require.False(t, exists)

	exists, err = drvr.CatalogExists(ctx, db, "definitely_not_a_db_xyz")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDriver_CreateAlterTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	tblName := stringz.UniqTableName("alter_test")
	tblDef := schema.NewTable(tblName, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	exists, err := drvr.TableExists(ctx, db, tblName)
	require.NoError(t, err)
	require.True(t, exists)

	// Add a column.
	require.NoError(t, drvr.AlterTableAddColumn(ctx, db, tblName, "extra", kind.Float))

	// Rename the new column.
	require.NoError(t, drvr.AlterTableRenameColumn(ctx, db, tblName, "extra", "extra2"))

	// Column types should reflect the three columns.
	colTypes, err := drvr.TableColumnTypes(ctx, db, tblName, nil)
	require.NoError(t, err)
	require.Len(t, colTypes, 3)

	// Subset, with the column name in a different case (fold resolution).
	colTypes, err = drvr.TableColumnTypes(ctx, db, tblName, []string{"NAME"})
	require.NoError(t, err)
	require.Len(t, colTypes, 1)

	// ListTableNames variants.
	names, err := drvr.ListTableNames(ctx, db, "", true, false)
	require.NoError(t, err)
	require.Contains(t, names, tblName)

	names, err = drvr.ListTableNames(ctx, db, "public", true, true)
	require.NoError(t, err)
	require.Contains(t, names, tblName)

	names, err = drvr.ListTableNames(ctx, db, "", false, false)
	require.NoError(t, err)
	require.Empty(t, names)

	// Rename the table.
	newName := stringz.UniqTableName("alter_test_renamed")
	require.NoError(t, drvr.AlterTableRename(ctx, db, tblName, newName))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(newName)) })

	exists, err = drvr.TableExists(ctx, db, newName)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestDriver_CopyTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	fromTbl := stringz.UniqTableName("copy_from")
	tblDef := schema.NewTable(fromTbl, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(fromTbl)) })

	const wantRows = 4
	for i := range wantRows {
		_, err := db.ExecContext(ctx, `INSERT INTO "`+fromTbl+`" (name, val) VALUES ($1, $2)`, "n", i)
		require.NoError(t, err)
	}

	// Copy with data.
	toTbl := stringz.UniqTableName("copy_to_data")
	affected, err := drvr.CopyTable(ctx, db, tablefq.From(fromTbl), tablefq.From(toTbl), true)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(toTbl)) })
	require.Equal(t, int64(wantRows), affected)

	// Copy without data.
	toTblEmpty := stringz.UniqTableName("copy_to_empty")
	affected, err = drvr.CopyTable(ctx, db, tablefq.From(fromTbl), tablefq.From(toTblEmpty), false)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(toTblEmpty)) })
	require.Zero(t, affected)

	var count int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+toTblEmpty+`"`).Scan(&count))
	require.Zero(t, count)
}

func TestDriver_PrepareInsertStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	tblName := stringz.UniqTableName("insert_test")
	tblDef := schema.NewTable(tblName, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { assert.NoError(t, conn.Close()) }()

	execer, err := drvr.PrepareInsertStmt(ctx, conn, tblName, []string{"name", "val"}, 1)
	require.NoError(t, err)

	rec := []any{"alice", int64(42)}
	require.NoError(t, execer.Munge(rec))
	affected, err := execer.Exec(ctx, rec...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, execer.Close())

	var got string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT name FROM "`+tblName+`" WHERE val = 42`).Scan(&got))
	require.Equal(t, "alice", got)
}

func TestDriver_PrepareUpdateStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	tblName := stringz.UniqTableName("update_test")
	tblDef := schema.NewTable(tblName, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	_, err := db.ExecContext(ctx, `INSERT INTO "`+tblName+`" (name, val) VALUES ($1, $2)`, "old", 1)
	require.NoError(t, err)

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { assert.NoError(t, conn.Close()) }()

	execer, err := drvr.PrepareUpdateStmt(ctx, conn, tblName, []string{"name"}, "val = 1")
	require.NoError(t, err)

	rec := []any{"new"}
	require.NoError(t, execer.Munge(rec))
	affected, err := execer.Exec(ctx, rec...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, execer.Close())

	var got string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT name FROM "`+tblName+`" WHERE val = 1`).Scan(&got))
	require.Equal(t, "new", got)
}

// TestDriver_Open_WithSchema exercises doOpen's search_path branch, which
// fires only when src.Schema is set: the connection's search_path is
// prefixed with the requested schema so unqualified names resolve there.
func TestDriver_Open_WithSchema(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	schemaName := stringz.UniqSuffix("open_schema")
	require.NoError(t, drvr.CreateSchema(ctx, db, schemaName))
	t.Cleanup(func() { assert.NoError(t, drvr.DropSchema(ctx, db, schemaName)) })

	src2 := th.Source(sakila.Pg).Clone()
	src2.Handle += "_schematest"
	src2.Schema = schemaName

	grip2, err := drvr.Open(ctx, src2, driver.ModeReadWrite)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, grip2.Close()) })

	db2, err := grip2.DB(ctx)
	require.NoError(t, err)

	cur, err := drvr.CurrentSchema(ctx, db2)
	require.NoError(t, err)
	require.Equal(t, schemaName, cur, "search_path should resolve to the requested schema")
}

// TestDriver_Metadata_IncomingFK_UniqueConstraint inspects the parent side
// of an FK relationship (to exercise the incoming-FK loader) and a table
// carrying an explicit UNIQUE constraint (to exercise the unique-constraint
// loader), both of which the other metadata tests leave on their zero path.
func TestDriver_Metadata_IncomingFK_UniqueConstraint(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)
	ctx := th.Context

	parent := stringz.UniqTableName("inc_parent")
	child := stringz.UniqTableName("inc_child")
	_, err := db.ExecContext(ctx,
		"CREATE TABLE "+parent+" (id INT PRIMARY KEY, code TEXT UNIQUE)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(ctx,
		"CREATE TABLE "+child+" (parent_id INT REFERENCES "+parent+" (id))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

	md, err := th.Open(src).TableMetadata(ctx, parent)
	require.NoError(t, err)

	// Parent has one incoming FK (from child) and one UNIQUE constraint (code).
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Incoming, 1)
	require.Equal(t, child, md.FK.Incoming[0].Table)
	require.NotEmpty(t, md.UniqueConstraints, "the UNIQUE(code) constraint should be reported")
}

// TestDriver_ErrorPaths drives the DB-error branches of the read/DDL
// methods through a deliberately-closed connection, confirming each
// surfaces the underlying error rather than swallowing it.
func TestDriver_ErrorPaths(t *testing.T) {
	tu.SkipShort(t, true)

	th, _, drvr, _, _ := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	src2 := th.Source(sakila.Pg).Clone()
	src2.Handle += "_errpath"
	grip2, err := drvr.Open(ctx, src2, driver.ModeReadWrite)
	require.NoError(t, err)
	db, err := grip2.DB(ctx)
	require.NoError(t, err)
	require.NoError(t, grip2.Close()) // db is now closed; every query must fail.

	t.Run("CurrentSchema", func(t *testing.T) {
		_, err := drvr.CurrentSchema(ctx, db)
		require.Error(t, err)
	})
	t.Run("CurrentCatalog", func(t *testing.T) {
		_, err := drvr.CurrentCatalog(ctx, db)
		require.Error(t, err)
	})
	t.Run("ListCatalogs", func(t *testing.T) {
		_, err := drvr.ListCatalogs(ctx, db)
		require.Error(t, err)
	})
	t.Run("ListSchemas", func(t *testing.T) {
		_, err := drvr.ListSchemas(ctx, db)
		require.Error(t, err)
	})
	t.Run("ListSchemaMetadata", func(t *testing.T) {
		_, err := drvr.ListSchemaMetadata(ctx, db)
		require.Error(t, err)
	})
	t.Run("SchemaExists", func(t *testing.T) {
		_, err := drvr.SchemaExists(ctx, db, "public")
		require.Error(t, err)
	})
	t.Run("CatalogExists", func(t *testing.T) {
		_, err := drvr.CatalogExists(ctx, db, "sakila")
		require.Error(t, err)
	})
	t.Run("TableExists", func(t *testing.T) {
		_, err := drvr.TableExists(ctx, db, "actor")
		require.Error(t, err)
	})
	t.Run("ListTableNames", func(t *testing.T) {
		_, err := drvr.ListTableNames(ctx, db, "", true, true)
		require.Error(t, err)
	})
	t.Run("DBProperties", func(t *testing.T) {
		_, err := drvr.DBProperties(ctx, db)
		require.Error(t, err)
	})
	t.Run("TableColumnTypes", func(t *testing.T) {
		_, err := drvr.TableColumnTypes(ctx, db, "actor", nil)
		require.Error(t, err)
	})
	t.Run("CopyTable", func(t *testing.T) {
		_, err := drvr.CopyTable(ctx, db, tablefq.From("actor"), tablefq.From("actor_copy"), true)
		require.Error(t, err)
	})
	t.Run("DropTable", func(t *testing.T) {
		require.Error(t, drvr.DropTable(ctx, db, tablefq.From("actor"), false))
	})
	t.Run("CreateSchema", func(t *testing.T) {
		require.Error(t, drvr.CreateSchema(ctx, db, "x"))
	})
	t.Run("DropSchema", func(t *testing.T) {
		require.Error(t, drvr.DropSchema(ctx, db, "x"))
	})
	t.Run("AlterTableRename", func(t *testing.T) {
		require.Error(t, drvr.AlterTableRename(ctx, db, "actor", "actor2"))
	})
	t.Run("AlterTableRenameColumn", func(t *testing.T) {
		require.Error(t, drvr.AlterTableRenameColumn(ctx, db, "actor", "actor_id", "aid"))
	})
	t.Run("AlterTableAddColumn", func(t *testing.T) {
		require.Error(t, drvr.AlterTableAddColumn(ctx, db, "actor", "extra", kind.Int))
	})
	t.Run("PrepareInsertStmt", func(t *testing.T) {
		_, err := drvr.PrepareInsertStmt(ctx, db, "actor", []string{"actor_id"}, 1)
		require.Error(t, err)
	})
	t.Run("PrepareUpdateStmt", func(t *testing.T) {
		_, err := drvr.PrepareUpdateStmt(ctx, db, "actor", []string{"actor_id"}, "")
		require.Error(t, err)
	})
}

// TestDriver_TableColumnTypes_UnknownColumn pins that requesting a column
// that doesn't exist on the table is a resolution error (the cross-source
// --insert path that motivated ResolveTableColumnsFold).
func TestDriver_TableColumnTypes_UnknownColumn(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	_, err := drvr.TableColumnTypes(th.Context, db, sakila.TblActor, []string{"no_such_column"})
	require.Error(t, err)
}

// TestDriver_PrepareUpdateStmt_NoCols pins that preparing an update with no
// target columns is rejected by buildUpdateStmt.
func TestDriver_PrepareUpdateStmt_NoCols(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.Pg)
	_, err := drvr.PrepareUpdateStmt(th.Context, db, sakila.TblActor, nil, "")
	require.Error(t, err)
}

func TestDriver_NewBatchInsert(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.Pg)
	ctx := th.Context

	tblName := stringz.UniqTableName("batch_test")
	tblDef := schema.NewTable(tblName, []string{"name", "val"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(ctx, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { assert.NoError(t, conn.Close()) }()

	bi, err := drvr.NewBatchInsert(ctx, "insert", conn, src, tblName, []string{"name", "val"})
	require.NoError(t, err)
	require.NotNil(t, bi)

	rec := []any{"bob", int64(7)}
	require.NoError(t, bi.Munge(rec))
	bi.RecordCh <- rec
	close(bi.RecordCh)

	// ErrCh is closed when processing completes.
	for err := range bi.ErrCh {
		require.NoError(t, err)
	}
	require.Equal(t, int64(1), bi.Written())

	var count int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+tblName+`"`).Scan(&count))
	require.Equal(t, int64(1), count)
}
