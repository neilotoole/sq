package sqlserver_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// These tests exercise the SQL Server driver against a live database. They are
// skipped under -short. They complement the cross-driver conformance suite in
// libsq/driver, locking down SQL Server-specific behavior in this package.

func TestDriver_Ping_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.MS)
	require.NoError(t, drvr.Ping(th.Context, src, driver.ModeReadWrite))
}

func TestDriver_DBProperties_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)
	props, err := drvr.DBProperties(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, props)
	// SERVERPROPERTY values and sys.configurations are merged; spot-check a
	// well-known server property and a well-known configuration option.
	require.Contains(t, props, "ProductVersion")
	require.Contains(t, props, "max server memory (MB)")
}

func TestDriver_TableExists_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)

	exists, err := drvr.TableExists(th.Context, db, sakila.TblActor)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = drvr.TableExists(th.Context, db, stringz.UniqTableName("no_exist"))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDriver_CurrentSchemaAndCatalog_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)

	schma, err := drvr.CurrentSchema(th.Context, db)
	require.NoError(t, err)
	require.Equal(t, "dbo", schma)

	catalog, err := drvr.CurrentCatalog(th.Context, db)
	require.NoError(t, err)
	require.Equal(t, "sakila", catalog)
}

func TestDriver_CatalogExists_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)

	exists, err := drvr.CatalogExists(th.Context, db, "sakila")
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = drvr.CatalogExists(th.Context, db, "no_such_catalog_"+stringz.Uniq8())
	require.NoError(t, err)
	require.False(t, exists)

	// The empty catalog short-circuits to false without a query.
	exists, err = drvr.CatalogExists(th.Context, db, "")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDriver_ListCatalogs_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)

	catalogs, err := drvr.ListCatalogs(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, catalogs)
	// The current catalog is always first.
	require.Equal(t, "sakila", catalogs[0])
	require.Contains(t, catalogs, "master")
}

func TestDriver_ListSchemaMetadata_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)

	schemas, err := drvr.ListSchemaMetadata(th.Context, db)
	require.NoError(t, err)
	require.NotEmpty(t, schemas)

	var found bool
	for _, s := range schemas {
		require.Equal(t, "sakila", s.Catalog)
		if s.Name == "dbo" {
			found = true
		}
	}
	require.True(t, found, "expected to find the dbo schema")
}

func TestDriver_ListTableNames_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)
	ctx := th.Context

	tables, err := drvr.ListTableNames(ctx, db, "", true, false)
	require.NoError(t, err)
	require.Contains(t, tables, sakila.TblActor)

	views, err := drvr.ListTableNames(ctx, db, "", false, true)
	require.NoError(t, err)
	require.NotContains(t, views, sakila.TblActor)

	both, err := drvr.ListTableNames(ctx, db, "", true, true)
	require.NoError(t, err)
	require.Contains(t, both, sakila.TblActor)
	require.GreaterOrEqual(t, len(both), len(tables))

	// Explicit schema arg exercises the @p1 branch.
	scoped, err := drvr.ListTableNames(ctx, db, "dbo", true, false)
	require.NoError(t, err)
	require.Contains(t, scoped, sakila.TblActor)

	// Neither tables nor views returns an empty slice without a query.
	none, err := drvr.ListTableNames(ctx, db, "", false, false)
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestDriver_TableColumnTypes_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)
	ctx := th.Context

	// All columns.
	colTypes, err := drvr.TableColumnTypes(ctx, db, sakila.TblActor, nil)
	require.NoError(t, err)
	require.Equal(t, len(sakila.TblActorCols()), len(colTypes))

	// Explicit subset of columns.
	want := []string{"actor_id", "first_name"}
	colTypes, err = drvr.TableColumnTypes(ctx, db, sakila.TblActor, want)
	require.NoError(t, err)
	require.Equal(t, len(want), len(colTypes))
	require.Equal(t, "actor_id", colTypes[0].Name())
}

func TestDriver_AlterTableAddColumn_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)

	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tbl, "col_extra", kind.Decimal))

	colTypes, err := drvr.TableColumnTypes(th.Context, db, tbl, []string{"col_extra"})
	require.NoError(t, err)
	require.Len(t, colTypes, 1)
}

func TestDriver_AlterTableRename_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)

	newName := stringz.UniqTableName("actor_renamed")
	require.NoError(t, drvr.AlterTableRename(th.Context, db, tbl, newName))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(newName)) })

	exists, err := drvr.TableExists(th.Context, db, newName)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestDriver_AlterTableRenameColumn_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, false)

	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, tbl, "first_name", "given_name"))

	colTypes, err := drvr.TableColumnTypes(th.Context, db, tbl, []string{"given_name"})
	require.NoError(t, err)
	require.Len(t, colTypes, 1)
}

func TestDriver_Truncate_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.MS)

	// Truncate without reseed.
	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)
	affected, err := drvr.Truncate(th.Context, src, tbl, false)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), affected)

	// Truncate with reseed on a table that has an identity column.
	tbl2 := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)
	affected, err = drvr.Truncate(th.Context, src, tbl2, true)
	require.NoError(t, err)
	require.Equal(t, int64(sakila.TblActorCount), affected)
}

// TestDriver_Truncate_NoIdentity_MSSQL exercises the branch where reseed is
// requested but the table has no identity column: the driver logs a warning
// and returns successfully rather than erroring.
func TestDriver_Truncate_NoIdentity_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tblName := stringz.UniqTableName("trunc_noident")
	tblDef := schema.NewTable(
		tblName,
		[]string{"name", "val"},
		[]kind.Kind{kind.Text, kind.Int},
	)
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	th.Insert(src, tblName, []string{"name", "val"}, []any{"a", int64(1)}, []any{"b", int64(2)})

	affected, err := drvr.Truncate(th.Context, src, tblName, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), affected)
}

// TestDriver_QuotedView_RowCount_MSSQL exercises the view row-count fallback in
// getTableMetadata (issue #1027 P1): sp_spaceused returns a NULL row count for a
// VIEW, so the code falls back to SELECT COUNT(*) FROM <view>. That site
// previously interpolated the name with %q (Go backslash-quoting, not SQL
// identifier doubling), so a view whose name contains a double-quote built
// malformed SQL. Here the view name contains a double-quote, so metadata must
// still load and report the correct row count.
func TestDriver_QuotedView_RowCount_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, grip, db := testh.NewWith(t, sakila.MS)

	baseTbl := stringz.UniqTableName(`ba"se`)
	tblDef := schema.NewTable(baseTbl, []string{"id", "val"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })
	th.Insert(src, baseTbl, []string{"id", "val"}, []any{int64(1), "a"}, []any{int64(2), "b"})

	viewName := stringz.UniqTableName(`vi"ew`)
	qView := stringz.DoubleQuote(viewName)
	_, err := db.ExecContext(th.Context,
		`CREATE VIEW `+qView+` AS SELECT id, val FROM `+stringz.DoubleQuote(baseTbl))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP VIEW IF EXISTS `+qView)
	})

	md, err := grip.TableMetadata(th.Context, viewName)
	require.NoError(t, err, "view metadata must load for a quoted view name")
	require.Equal(t, viewName, md.Name)
	require.Equal(t, int64(2), md.RowCount, "row count must load via the COUNT(*) fallback")
}

// TestSourceMetadata_BrokenView_MSSQL pins that a metadata scan tolerates a
// view whose underlying object has vanished. A base table dropped mid-scan by
// concurrent DDL leaves its dependent views broken: sp_spaceused still
// succeeds (a view has no storage, so it reports a NULL row count), and then
// the SELECT COUNT(*) row-count fallback raises error 4413 ("binding
// errors"); a view that itself vanishes raises 208 from the same site.
// Previously only error 15009 was tolerated, so either failed the whole scan;
// this was the cause of TestDBSemver flakes under parallel test load (issue
// #1027). A broken view exists in the catalog, so it stays visible in the
// results, with no row count; an object that is gone (15009/208) is omitted.
// The dangling view used here reproduces the 4413 shape deterministically,
// with no race; 208 is pinned by Test_isObjectVanishedErr.
func TestSourceMetadata_BrokenView_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, grip, db := testh.NewWith(t, sakila.MS)

	baseTbl := stringz.UniqTableName("vanish_base")
	tblDef := schema.NewTable(baseTbl, []string{"id", "val"}, []kind.Kind{kind.Int, kind.Text})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })

	viewName := stringz.UniqTableName("vanish_view")
	qView := stringz.DoubleQuote(viewName)
	_, err := db.ExecContext(th.Context,
		`CREATE VIEW `+qView+` AS SELECT id, val FROM `+stringz.DoubleQuote(baseTbl))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP VIEW IF EXISTS `+qView)
	})

	// Drop the base table out from under the view: the view remains
	// enumerable, but its COUNT(*) now raises error 4413 (binding errors).
	require.NoError(t, drvr.DropTable(th.Context, db, tablefq.From(baseTbl), false))

	// Source-wide path: the scan succeeds, and the broken view stays visible
	// with no row count.
	md, err := grip.SourceMetadata(th.Context, false)
	require.NoError(t, err,
		"source scan must tolerate a view with binding errors (error 4413)")

	var foundView *metadata.Table
	for _, tbl := range md.Tables {
		if tbl.Name == viewName {
			foundView = tbl
			break
		}
	}
	require.NotNil(t, foundView, "the broken view stays visible in the results")
	require.Equal(t, int64(0), foundView.RowCount, "no row count for a broken view")

	// Per-table path: same behavior.
	tblMd, err := grip.TableMetadata(th.Context, viewName)
	require.NoError(t, err,
		"per-table metadata must tolerate a view with binding errors (error 4413)")
	require.Equal(t, viewName, tblMd.Name)
	require.Equal(t, int64(0), tblMd.RowCount, "no row count for a broken view")
}

// TestDriver_Truncate_QuotedIdentity_MSSQL exercises Truncate's reseed path
// (DBCC CHECKIDENT) against an identity table whose name contains a double
// quote. DBCC parses the name out of a string literal, so it must be
// bracket-quoted before single-quoting (#1027); a raw single-quoted name broke
// object resolution after the destructive DELETE.
func TestDriver_Truncate_QuotedIdentity_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tblName := stringz.UniqTableName(`tr"unc`)
	// sq's CreateTable does not emit IDENTITY columns, so create the identity
	// table with raw (properly quoted) DDL.
	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+stringz.DoubleQuote(tblName)+` (id INT IDENTITY(1,1) PRIMARY KEY, val NVARCHAR(20))`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })
	th.Insert(src, tblName, []string{"val"}, []any{"a"}, []any{"b"})

	affected, err := drvr.Truncate(th.Context, src, tblName, true)
	require.NoError(t, err, "Truncate+reset must handle a quoted identity table name")
	require.Equal(t, int64(2), affected)
}

// TestDriver_AlterTable_QuotedIdentifier_MSSQL exercises the ALTER-family DDL
// (AlterTableAddColumn, AlterTableRenameColumn, AlterTableRename) against a
// table and columns whose names contain a double quote (#1027). AddColumn
// quotes as an identifier; the sp_rename paths bracket-quote the name inside a
// string literal.
func TestDriver_AlterTable_QuotedIdentifier_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tblName := stringz.UniqTableName(`al"ter`)
	tblDef := schema.NewTable(tblName, []string{`c"1`, "id"}, []kind.Kind{kind.Text, kind.Int})
	require.NoError(t, drvr.CreateTable(th.Context, db, tblDef))
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

	require.NoError(t, drvr.AlterTableAddColumn(th.Context, db, tblName, `c"2`, kind.Int),
		"AlterTableAddColumn must escape quoted table/column names")
	require.NoError(t, drvr.AlterTableRenameColumn(th.Context, db, tblName, `c"1`, `c"1b`),
		"AlterTableRenameColumn must escape quoted names")

	newName := stringz.UniqTableName(`al"ter2`)
	require.NoError(t, drvr.AlterTableRename(th.Context, db, tblName, newName),
		"AlterTableRename must escape quoted names")
	t.Cleanup(func() { th.DropTable(src, tablefq.From(newName)) })

	md, err := th.Open(src).TableMetadata(th.Context, newName)
	require.NoError(t, err, "renamed quoted table must load")
	require.Equal(t, newName, md.Name)
	require.Len(t, md.Columns, 3, "expected c\"1b, id, c\"2 after the alters")
}

// TestDriver_DropSchema_QuotedIdentifier_MSSQL exercises DropSchema and
// genDropSchemaObjectsStmt against a schema whose name contains a single quote
// (#1027). DropSchema first enumerates and drops the schema's objects (the
// generated script assigns @SchemaName from a string literal, so the name must
// be single-quoted with the ' doubled), then drops the schema itself
// (identifier-quoted). A single quote is the char that breaks the generated
// script's string literal; a name containing a ] can still fail server-side
// because the generated script bracket-builds object names without doubling ]
// (a deeper pre-existing limit, out of scope here).
func TestDriver_DropSchema_QuotedIdentifier_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, _, drvr, _, db := testh.NewWith(t, sakila.MS)
	ctx := th.Context

	schemaName := `s'ch_` + stringz.Uniq8()
	require.NoError(t, drvr.CreateSchema(ctx, db, schemaName),
		"CreateSchema must escape a quoted schema name")
	dropped := false
	t.Cleanup(func() {
		if !dropped {
			_ = drvr.DropSchema(ctx, db, schemaName)
		}
	})

	// Put a table in the schema so genDropSchemaObjectsStmt has an object to
	// drop before DROP SCHEMA (it enumerates sys.tables for the schema).
	_, err := db.ExecContext(ctx,
		`CREATE TABLE `+stringz.DoubleQuote(schemaName)+`.`+stringz.DoubleQuote(`t"1`)+` (id INT)`)
	require.NoError(t, err)

	require.NoError(t, drvr.DropSchema(ctx, db, schemaName),
		"DropSchema must escape a quoted schema name and its objects")
	dropped = true
}

func TestDriver_PrepareUpdateStmt_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

	destCols := []string{"first_name", "last_name"}
	wantVals := []any{"Kubla", "Khan"}

	execer, err := drvr.PrepareUpdateStmt(th.Context, db, tbl, destCols, "actor_id = ?")
	require.NoError(t, err)
	require.Equal(t, destCols, execer.DestMeta().Names())
	require.NoError(t, execer.Munge(wantVals))

	affected, err := execer.Exec(th.Context, append(wantVals, int64(1))...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
}

// TestDriver_PrepareInsertStmt_IdentityInsert_MSSQL exercises the
// identity-insert recovery path: inserting an explicit value into an identity
// column first fails with errCodeIdentityInsert, after which the driver sets
// IDENTITY_INSERT ON and retries successfully.
func TestDriver_PrepareInsertStmt_IdentityInsert_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, db := testh.NewWith(t, sakila.MS)

	// SELECT * INTO preserves the identity property of actor_id.
	tbl := th.CopyTable(true, src, tablefq.From(sakila.TblActor), tablefq.T{}, true)

	destCols := []string{"actor_id", "first_name", "last_name", "last_update"}
	const explicitID int64 = 99999
	vals := []any{explicitID, "Ada", "Lovelace", time.Now()}

	// IDENTITY_INSERT is connection-scoped, so the SET statement issued by the
	// driver's recovery path and the prepared insert must share one connection.
	// Use a transaction (a single connection) as the driver does for real
	// batch inserts; passing the *sql.DB pool would run them on different
	// connections and the retry would still see IDENTITY_INSERT OFF.
	tx, err := db.BeginTx(th.Context, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	execer, err := drvr.PrepareInsertStmt(th.Context, tx, tbl, destCols, 1)
	require.NoError(t, err)
	require.NoError(t, execer.Munge(vals))

	affected, err := execer.Exec(th.Context, vals...)
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.NoError(t, tx.Commit())

	sink, err := th.QuerySQL(src, nil, "SELECT * FROM "+tbl+" WHERE actor_id = 99999")
	require.NoError(t, err)
	require.Equal(t, 1, len(sink.Recs))
}

// TestDriver_ErrorPaths_ClosedDB_MSSQL drives the DB-error branches of the
// driver's db-backed methods by issuing them against a closed connection. Every
// query/exec fails with "database is closed", exercising the errw wrapping that
// the happy-path tests don't reach.
func TestDriver_ErrorPaths_ClosedDB_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.MS)
	ctx := th.Context

	// Open a dedicated grip so closing it doesn't disturb the shared helper db.
	grip, err := drvr.Open(ctx, src, driver.ModeReadWrite)
	require.NoError(t, err)
	db, err := grip.DB(ctx)
	require.NoError(t, err)
	require.NoError(t, grip.Close())

	const bogusTbl = "no_such_table_xyz"
	fqTbl := tablefq.From(bogusTbl)

	_, err = drvr.DBProperties(ctx, db)
	assert.Error(t, err)
	_, err = drvr.TableExists(ctx, db, bogusTbl)
	assert.Error(t, err)
	_, err = drvr.CurrentSchema(ctx, db)
	assert.Error(t, err)
	_, err = drvr.CurrentCatalog(ctx, db)
	assert.Error(t, err)
	// Non-empty names get past the short-circuit and reach the query.
	_, err = drvr.SchemaExists(ctx, db, "dbo")
	assert.Error(t, err)
	_, err = drvr.CatalogExists(ctx, db, "sakila")
	assert.Error(t, err)
	_, err = drvr.ListSchemas(ctx, db)
	assert.Error(t, err)
	_, err = drvr.ListSchemaMetadata(ctx, db)
	assert.Error(t, err)
	_, err = drvr.ListCatalogs(ctx, db)
	assert.Error(t, err)
	_, err = drvr.ListTableNames(ctx, db, "", true, true)
	assert.Error(t, err)
	_, err = drvr.TableColumnTypes(ctx, db, bogusTbl, nil)
	assert.Error(t, err)
	assert.Error(t, drvr.CreateSchema(ctx, db, "sch_"+stringz.Uniq8()))
	assert.Error(t, drvr.DropSchema(ctx, db, "sch_"+stringz.Uniq8()))
	assert.Error(t, drvr.AlterTableAddColumn(ctx, db, bogusTbl, "c", kind.Int))
	assert.Error(t, drvr.AlterTableRename(ctx, db, bogusTbl, "c"))
	assert.Error(t, drvr.AlterTableRenameColumn(ctx, db, bogusTbl, "c", "d"))
	_, err = drvr.CopyTable(ctx, db, fqTbl, tablefq.From("dst"), true)
	assert.Error(t, err)
	assert.Error(t, drvr.DropTable(ctx, db, fqTbl, false))
	_, err = drvr.PrepareInsertStmt(ctx, db, bogusTbl, []string{"c"}, 1)
	assert.Error(t, err)
	_, err = drvr.PrepareUpdateStmt(ctx, db, bogusTbl, []string{"c"}, "c = ?")
	assert.Error(t, err)

	tblDef := schema.NewTable("t_"+stringz.Uniq8(), []string{"c"}, []kind.Kind{kind.Int})
	assert.Error(t, drvr.CreateTable(ctx, db, tblDef))

	// Truncate opens its own connection, so target a non-existent table to
	// drive its delete-failure branch instead.
	_, err = drvr.Truncate(ctx, src, bogusTbl, false)
	assert.Error(t, err)
}

// TestDriver_Open_BadLocation_MSSQL exercises the error path of Open when the
// source location can't be connected to.
func TestDriver_Open_BadLocation_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.MS)

	bad := src.Clone()
	bad.Location = "sqlserver://sq:wrongpw@localhost:1433?database=sakila"
	_, err := drvr.Open(th.Context, bad, driver.ModeReadWrite)
	require.Error(t, err)
}

// TestDriver_Open_WithCatalog_MSSQL covers the doOpen branch that rewrites the
// connection string when the source specifies a catalog.
func TestDriver_Open_WithCatalog_MSSQL(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th, src, drvr, _, _ := testh.NewWith(t, sakila.MS)

	withCat := src.Clone()
	withCat.Catalog = "sakila"
	grip, err := drvr.Open(th.Context, withCat, driver.ModeReadWrite)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, grip.Close()) })

	catalog, err := drvr.CurrentCatalog(th.Context, th.OpenDB(withCat))
	require.NoError(t, err)
	require.Equal(t, "sakila", catalog)
}
