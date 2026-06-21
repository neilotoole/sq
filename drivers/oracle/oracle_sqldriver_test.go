package oracle_test

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// sakilaSource bundles the testh fixtures for the Oracle source.
type sakilaSource struct {
	src  *source.Source
	db   *sql.DB
	drvr driver.SQLDriver
}

// newOracleTable creates a fresh upper-cased table with (ID NUMBER, NAME
// VARCHAR2) and registers cleanup. It returns the stored (upper-case) table
// name.
func newOracleTable(t *testing.T, th *testh.Helper, s *sakilaSource, prefix string) string {
	t.Helper()
	tblName := strings.ToUpper(stringz.UniqTableName(prefix))
	tblDef := &schema.Table{
		Name: tblName,
		Cols: []*schema.Column{
			{Name: "ID", Kind: kind.Int, NotNull: true},
			{Name: "NAME", Kind: kind.Text},
		},
	}
	require.NoError(t, s.drvr.CreateTable(th.Context, s.db, tblDef))
	t.Cleanup(func() { th.DropTable(s.src, tablefq.From(tblName)) })
	return tblName
}

// requireOracle returns the standard testh fixtures for the Oracle Sakila
// source, skipping when it isn't configured.
func requireOracle(t *testing.T) (*testh.Helper, *sakilaSource) {
	t.Helper()
	tu.SkipShort(t, true)
	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	grip := th.Open(src)
	db, err := grip.DB(th.Context)
	require.NoError(t, err)
	return th, &sakilaSource{src: src, db: db, drvr: grip.SQLDriver()}
}

// TestDriver_Truncate covers Truncate for both the reset (DROP STORAGE) and
// non-reset (REUSE STORAGE) paths, asserting the pre-truncate row count is
// returned.
func TestDriver_Truncate(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	for _, reset := range []bool{true, false} {
		name := "reuse"
		if reset {
			name = "reset"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tbl := newOracleTable(t, th, s, "trunc")
			th.Insert(s.src, tbl, []string{"ID", "NAME"},
				[]any{1, "a"}, []any{2, "b"}, []any{3, "c"})

			affected, err := s.drvr.Truncate(th.Context, s.src, tbl, reset)
			require.NoError(t, err)
			require.Equal(t, int64(3), affected, "Truncate reports pre-truncate count")
			require.Equal(t, int64(0), th.RowCount(s.src, tbl))
		})
	}
}

// TestDriver_CopyTable covers both the schema-only (copyData=false) and
// data-copy (copyData=true) paths.
func TestDriver_CopyTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	src := newOracleTable(t, th, s, "copysrc")
	th.Insert(s.src, src, []string{"ID", "NAME"}, []any{1, "a"}, []any{2, "b"})

	t.Run("schema_only", func(t *testing.T) {
		t.Parallel()
		dst := strings.ToUpper(stringz.UniqTableName("copydst_s"))
		t.Cleanup(func() { th.DropTable(s.src, tablefq.From(dst)) })
		affected, err := s.drvr.CopyTable(th.Context, s.db, tablefq.From(src), tablefq.From(dst), false)
		require.NoError(t, err)
		require.Equal(t, int64(0), affected)
		require.Equal(t, int64(0), th.RowCount(s.src, dst), "structure copied, no rows")
	})

	t.Run("with_data", func(t *testing.T) {
		t.Parallel()
		dst := strings.ToUpper(stringz.UniqTableName("copydst_d"))
		t.Cleanup(func() { th.DropTable(s.src, tablefq.From(dst)) })
		_, err := s.drvr.CopyTable(th.Context, s.db, tablefq.From(src), tablefq.From(dst), true)
		require.NoError(t, err)
		require.Equal(t, int64(2), th.RowCount(s.src, dst), "all rows copied")
	})
}

// TestDriver_AlterTable covers AlterTableAddColumn, AlterTableRenameColumn, and
// AlterTableRename in sequence on a single table.
func TestDriver_AlterTable(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	tbl := newOracleTable(t, th, s, "alter")

	require.NoError(t, s.drvr.AlterTableAddColumn(th.Context, s.db, tbl, "EXTRA", kind.Decimal))
	require.NoError(t, s.drvr.AlterTableRenameColumn(th.Context, s.db, tbl, "EXTRA", "EXTRA2"))

	colTypes, err := s.drvr.TableColumnTypes(th.Context, s.db, tbl, nil)
	require.NoError(t, err)
	var names []string
	for _, ct := range colTypes {
		names = append(names, ct.Name())
	}
	require.Contains(t, names, "EXTRA2")
	require.NotContains(t, names, "EXTRA")

	newName := strings.ToUpper(stringz.UniqTableName("alter_renamed"))
	require.NoError(t, s.drvr.AlterTableRename(th.Context, s.db, tbl, newName))
	t.Cleanup(func() { th.DropTable(s.src, tablefq.From(newName)) })

	exists, err := s.drvr.TableExists(th.Context, s.db, newName)
	require.NoError(t, err)
	require.True(t, exists)
}

// TestDriver_TableColumnTypes covers both the explicit-column path and the
// empty-column path (which routes through getTableColumnNames).
func TestDriver_TableColumnTypes(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)
	tbl := newOracleTable(t, th, s, "coltypes")

	all, err := s.drvr.TableColumnTypes(th.Context, s.db, tbl, nil)
	require.NoError(t, err)
	require.Len(t, all, 2, "empty colNames returns all columns")

	subset, err := s.drvr.TableColumnTypes(th.Context, s.db, tbl, []string{"NAME"})
	require.NoError(t, err)
	require.Len(t, subset, 1)
	require.Equal(t, "NAME", subset[0].Name())
}

// TestDriver_PrepareUpdateStmt covers PrepareUpdateStmt with a WHERE clause
// (exercising oracleSubstWherePlaceholders) and without one.
func TestDriver_PrepareUpdateStmt(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	t.Run("with_where", func(t *testing.T) {
		t.Parallel()
		tbl := newOracleTable(t, th, s, "upd_w")
		th.Insert(s.src, tbl, []string{"ID", "NAME"}, []any{1, "a"}, []any{2, "b"})

		execer, err := s.drvr.PrepareUpdateStmt(th.Context, s.db, tbl, []string{"NAME"}, "ID = ?")
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, execer.Close()) })

		vals := []any{"updated"}
		require.NoError(t, execer.Munge(vals))
		affected, err := execer.Exec(th.Context, append(vals, 1)...)
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)
	})

	t.Run("no_where", func(t *testing.T) {
		t.Parallel()
		tbl := newOracleTable(t, th, s, "upd_n")
		th.Insert(s.src, tbl, []string{"ID", "NAME"}, []any{1, "a"}, []any{2, "b"})

		execer, err := s.drvr.PrepareUpdateStmt(th.Context, s.db, tbl, []string{"NAME"}, "")
		require.NoError(t, err)
		t.Cleanup(func() { assert.NoError(t, execer.Close()) })

		vals := []any{"all"}
		require.NoError(t, execer.Munge(vals))
		affected, err := execer.Exec(th.Context, vals...)
		require.NoError(t, err)
		require.Equal(t, int64(2), affected, "no WHERE updates every row")
	})
}

// TestDriver_Schemas covers ListSchemas, SchemaExists (present and absent), and
// ListSchemaMetadata.
func TestDriver_Schemas(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	schemas, err := s.drvr.ListSchemas(th.Context, s.db)
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	require.Contains(t, schemas, "SAKILA")

	exists, err := s.drvr.SchemaExists(th.Context, s.db, "sakila")
	require.NoError(t, err)
	require.True(t, exists, "SchemaExists is case-insensitive")

	exists, err = s.drvr.SchemaExists(th.Context, s.db, "no_such_schema_xyz")
	require.NoError(t, err)
	require.False(t, exists)

	mds, err := s.drvr.ListSchemaMetadata(th.Context, s.db)
	require.NoError(t, err)
	require.NotEmpty(t, mds)
	var found bool
	for _, md := range mds {
		if md.Name == "SAKILA" {
			found = true
		}
	}
	require.True(t, found, "ListSchemaMetadata includes SAKILA")
}

// TestDriver_DropTable_IfExists covers the ifExists=true path against a
// nonexistent table (exercising isErrTableNotExist) and the normal drop.
func TestDriver_DropTable_IfExists(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	missing := strings.ToUpper(stringz.UniqTableName("nope"))
	require.NoError(t, s.drvr.DropTable(th.Context, s.db, tablefq.From(missing), true),
		"dropping a missing table with ifExists=true must be a no-op")

	err := s.drvr.DropTable(th.Context, s.db, tablefq.From(missing), false)
	require.Error(t, err, "dropping a missing table with ifExists=false must error")
}

// TestDriver_SourceMetadata_MaterializedView is the regression test for the
// MV duplicate-listing bug: an Oracle materialized view's container table
// appears in USER_TABLES under the MV's own name, so the source-metadata
// loader previously emitted the object twice (once as TABLE, once as
// MATERIALIZED VIEW). After the fix it must appear exactly once, classified
// as a materialized view.
func TestDriver_SourceMetadata_MaterializedView(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	base := newOracleTable(t, th, s, "mvbase")
	th.Insert(s.src, base, []string{"ID", "NAME"}, []any{1, "a"}, []any{2, "b"})

	mvName := strings.ToUpper(stringz.UniqTableName("mvdup"))
	_, err := s.db.ExecContext(th.Context,
		`CREATE MATERIALIZED VIEW "`+mvName+`" AS SELECT "ID", "NAME" FROM "`+base+`"`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(th.Context, `DROP MATERIALIZED VIEW "`+mvName+`"`)
	})

	md, err := th.SourceMetadata(s.src)
	require.NoError(t, err)

	var count int
	var dbType string
	for _, tbl := range md.Tables {
		if tbl.Name == mvName {
			count++
			dbType = tbl.DBTableType
		}
	}
	require.Equal(t, 1, count, "materialized view must appear exactly once, not duplicated")
	require.Equal(t, "MATERIALIZED VIEW", dbType)

	// The single-object path must also resolve it as a materialized view.
	mvMeta, err := th.Open(s.src).TableMetadata(th.Context, mvName)
	require.NoError(t, err)
	require.Equal(t, "MATERIALIZED VIEW", mvMeta.DBTableType)
	require.Equal(t, int64(2), mvMeta.RowCount)
}

// TestDriver_ListTableNames_Variants covers the views-only and tables+views
// paths plus the empty-schema path (which falls back to CurrentSchema).
func TestDriver_ListTableNames_Variants(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	// Empty schema → CurrentSchema fallback; views only.
	views, err := s.drvr.ListTableNames(th.Context, s.db, "", false, true)
	require.NoError(t, err)
	require.Contains(t, views, "CUSTOMER_LIST", "Sakila ships the customer_list view")

	tablesOnly, err := s.drvr.ListTableNames(th.Context, s.db, "SAKILA", true, false)
	require.NoError(t, err)
	require.Contains(t, tablesOnly, "ACTOR")
	require.NotContains(t, tablesOnly, "CUSTOMER_LIST", "views excluded when views=false")

	both, err := s.drvr.ListTableNames(th.Context, s.db, "SAKILA", true, true)
	require.NoError(t, err)
	require.Contains(t, both, "ACTOR")
	require.Contains(t, both, "CUSTOMER_LIST")
	require.True(t, len(both) > len(tablesOnly), "tables+views is a superset of tables")
}

// TestDriver_QueryAggregates is a package-local regression for the computed-
// NUMBER kind handling (issues #594, #839, #844): count() must scan as an
// integer (not a decimal string), sum() stays decimal, and avg() is float.
// It exercises the renderer function overrides and RecordMeta's kindHints at
// runtime.
func TestDriver_QueryAggregates(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, _ := requireOracle(t)

	t.Run("count", func(t *testing.T) {
		t.Parallel()
		sink, err := th.QuerySLQ(sakila.Ora+".actor | count()", nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 1)
		// gh844: must surface as an integer, not a decimal string "200".
		require.EqualValues(t, sakila.TblActorCount, stringz.Val(sink.Recs[0][0]))
	})

	t.Run("sum_and_avg", func(t *testing.T) {
		t.Parallel()
		sink, err := th.QuerySLQ(sakila.Ora+`.payment | sum(.amount)`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 1)
		require.NotNil(t, sink.Recs[0][0], "sum(.amount) must scan without an int64 crash")

		sink, err = th.QuerySLQ(sakila.Ora+`.payment | avg(.amount)`, nil)
		require.NoError(t, err)
		require.Len(t, sink.Recs, 1)
		require.NotNil(t, sink.Recs[0][0])
	})
}

// TestDriver_ErrorPaths exercises the error-return branches of the DB-backed
// methods by handing them a closed *sql.DB. Every query fails immediately with
// "sql: database is closed", so this drives the errw(err) propagation paths
// deterministically without needing fault injection at the wire level.
func TestDriver_ErrorPaths(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	closedDB, err := sql.Open("oracle", s.src.Location)
	require.NoError(t, err)
	require.NoError(t, closedDB.Close())

	ctx := th.Context

	_, err = s.drvr.CurrentSchema(ctx, closedDB)
	require.Error(t, err)

	_, err = s.drvr.ListSchemas(ctx, closedDB)
	require.Error(t, err)

	_, err = s.drvr.SchemaExists(ctx, closedDB, "SAKILA")
	require.Error(t, err)

	_, err = s.drvr.TableExists(ctx, closedDB, "ACTOR")
	require.Error(t, err)

	_, err = s.drvr.ListTableNames(ctx, closedDB, "SAKILA", true, true)
	require.Error(t, err)

	_, err = s.drvr.DBProperties(ctx, closedDB)
	require.Error(t, err)

	// Explicit column list: the SELECT ... WHERE 1=0 fails.
	_, err = s.drvr.TableColumnTypes(ctx, closedDB, "ACTOR", []string{"ACTOR_ID"})
	require.Error(t, err)

	// Empty column list: getTableColumnNames fails first.
	_, err = s.drvr.TableColumnTypes(ctx, closedDB, "ACTOR", nil)
	require.Error(t, err)

	_, err = s.drvr.ListSchemaMetadata(ctx, closedDB)
	require.Error(t, err)

	require.Error(t, s.drvr.CreateTable(ctx, closedDB,
		&schema.Table{Name: "X", Cols: []*schema.Column{{Name: "ID", Kind: kind.Int}}}))

	// ifExists=true but the error is "database closed", not ORA-00942, so it
	// must still propagate rather than be swallowed.
	require.Error(t, s.drvr.DropTable(ctx, closedDB, tablefq.From("ACTOR"), true))

	require.Error(t, s.drvr.AlterTableAddColumn(ctx, closedDB, "ACTOR", "C", kind.Int))
	require.Error(t, s.drvr.AlterTableRename(ctx, closedDB, "ACTOR", "ACTOR2"))
	require.Error(t, s.drvr.AlterTableRenameColumn(ctx, closedDB, "ACTOR", "A", "B"))

	_, err = s.drvr.PrepareInsertStmt(ctx, closedDB, "ACTOR", []string{"ACTOR_ID"}, 1)
	require.Error(t, err)

	_, err = s.drvr.PrepareUpdateStmt(ctx, closedDB, "ACTOR", []string{"FIRST_NAME"}, "ACTOR_ID = ?")
	require.Error(t, err)

	_, err = s.drvr.CopyTable(ctx, closedDB, tablefq.From("ACTOR"), tablefq.From("ACTOR_COPY"), true)
	require.Error(t, err)
}

// TestDriver_ListTableNames_MaterializedView verifies ListTableNames does not
// return a materialized view's name twice (once from ALL_TABLES, once from
// ALL_MVIEWS).
func TestDriver_ListTableNames_MaterializedView(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()
	th, s := requireOracle(t)

	base := newOracleTable(t, th, s, "lmvbase")
	mvName := strings.ToUpper(stringz.UniqTableName("lmvdup"))
	_, err := s.db.ExecContext(th.Context,
		`CREATE MATERIALIZED VIEW "`+mvName+`" AS SELECT "ID", "NAME" FROM "`+base+`"`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(th.Context, `DROP MATERIALIZED VIEW "`+mvName+`"`)
	})

	names, err := s.drvr.ListTableNames(th.Context, s.db, "SAKILA", true, false)
	require.NoError(t, err)

	var n int
	for _, name := range names {
		if name == mvName {
			n++
		}
	}
	require.Equal(t, 1, n, "materialized view name must not be listed twice")

	// tables=false && views=false short-circuits to an empty slice.
	empty, err := s.drvr.ListTableNames(th.Context, s.db, "", false, false)
	require.NoError(t, err)
	require.Empty(t, empty)
}
