package sqlserver_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

// TestInspect_ColumnFlags_SQLServer verifies that IDENTITY columns, computed
// columns, and columns with an explicit COLLATE clause are correctly reflected
// in table metadata.
func TestInspect_ColumnFlags_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			tblName := stringz.UniqTableName("col_flags")
			// Computed column expression uses COLLATE DATABASE_DEFAULT to resolve
			// the collation conflict between Latin1_General_BIN (first_name) and
			// the server's default collation (last_name and the string literal).
			_, err := db.ExecContext(th.Context, `CREATE TABLE `+tblName+` (
				id         INT          IDENTITY(1,1) NOT NULL PRIMARY KEY,
				first_name NVARCHAR(50) COLLATE Latin1_General_BIN NOT NULL,
				last_name  NVARCHAR(50) NOT NULL,
				full_name  AS (first_name COLLATE DATABASE_DEFAULT + N' ' + last_name)
			)`)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

			md, err := th.Open(src).TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Len(t, md.Columns, 4)

			// id → IDENTITY
			idCol := md.Columns[0]
			require.Equal(t, "id", idCol.Name)
			require.True(t, idCol.Identity, "id column must be Identity=true")
			require.False(t, idCol.Generated, "id column must not be Generated")

			// first_name → explicit collation
			fnCol := md.Columns[1]
			require.Equal(t, "first_name", fnCol.Name)
			require.Equal(t, "Latin1_General_BIN", fnCol.Collation,
				"first_name must carry its explicit Latin1_General_BIN collation")

			// full_name → computed/generated
			fqCol := md.Columns[3]
			require.Equal(t, "full_name", fqCol.Name)
			require.True(t, fqCol.Generated, "full_name must be Generated=true")
			require.NotEmpty(t, fqCol.GeneratedExpr, "full_name must have a non-empty GeneratedExpr")
		})
	}
}

// TestInspect_CheckConstraints_SQLServer verifies that CHECK constraints
// are reported in table metadata with the engine-formatted clause text.
func TestInspect_CheckConstraints_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			tblName := stringz.UniqTableName("chk_test")
			// SQL Server constraint names are database-scoped (not table-scoped),
			// so we use a unique name to avoid conflicts across parallel test runs.
			checkName := stringz.UniqTableName("chk_age")
			_, err := db.ExecContext(th.Context, `CREATE TABLE `+tblName+` (
				id  INT NOT NULL PRIMARY KEY,
				age INT NOT NULL,
				CONSTRAINT `+checkName+` CHECK (age >= 0)
			)`)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

			md, err := th.Open(src).TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Len(t, md.CheckConstraints, 1,
				"expected exactly one check constraint")
			cc := md.CheckConstraints[0]
			require.Equal(t, checkName, cc.Name)
			require.NotEmpty(t, cc.Clause, "check constraint clause must be non-empty")
		})
	}
}

// TestInspect_Triggers_SQLServer verifies that DML triggers attached to a
// table are reported with correct timing, events, and enabled state.
func TestInspect_Triggers_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			tblName := stringz.UniqTableName("trig_test")
			trigName := tblName + "_trig"
			_, err := db.ExecContext(th.Context, `CREATE TABLE `+tblName+` (
				id  INT NOT NULL PRIMARY KEY,
				val INT NOT NULL
			)`)
			require.NoError(t, err)
			// Trigger is dropped automatically when its parent table is dropped;
			// no separate trigger cleanup is needed.
			t.Cleanup(func() { th.DropTable(src, tablefq.From(tblName)) })

			_, err = db.ExecContext(th.Context, `CREATE TRIGGER `+trigName+`
ON `+tblName+` AFTER INSERT
AS BEGIN
    SELECT 1
END`)
			require.NoError(t, err)

			md, err := th.Open(src).TableMetadata(th.Context, tblName)
			require.NoError(t, err)
			require.Len(t, md.Triggers, 1)
			tr := md.Triggers[0]
			require.Equal(t, trigName, tr.Name)
			require.Equal(t, "AFTER", tr.Timing)
			require.Contains(t, tr.Events, "INSERT")
			require.NotNil(t, tr.Enabled, "SQL Server triggers have an enabled/disabled state")
			require.True(t, *tr.Enabled, "newly-created trigger must be enabled")
			require.NotEmpty(t, tr.Definition)
		})
	}
}

// TestInspect_ViewInsteadOfTrigger_SQLServer verifies that an INSTEAD OF
// trigger on a view is returned by the per-table inspect path
// (Grip.TableMetadata), matching the behavior of source-wide inspect.
func TestInspect_ViewInsteadOfTrigger_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			baseTbl := stringz.UniqTableName("viot_base")
			viewName := stringz.UniqTableName("viot_view")
			trigName := viewName + "_trig"

			// Register base-table cleanup FIRST (LIFO: view cleaned up first).
			_, err := db.ExecContext(th.Context,
				`CREATE TABLE `+baseTbl+` (id INT NOT NULL PRIMARY KEY)`)
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })

			_, err = db.ExecContext(th.Context,
				`CREATE VIEW `+viewName+` AS SELECT id FROM `+baseTbl)
			require.NoError(t, err)
			// Register view cleanup SECOND (LIFO → runs before base-table cleanup).
			t.Cleanup(func() {
				_, _ = db.ExecContext(th.Context, `DROP VIEW `+viewName)
			})

			_, err = db.ExecContext(th.Context, `CREATE TRIGGER `+trigName+
				` ON `+viewName+` INSTEAD OF INSERT AS BEGIN SET NOCOUNT ON; END`)
			require.NoError(t, err)
			// Dropping the view drops its INSTEAD OF triggers automatically.

			md, err := th.Open(src).TableMetadata(th.Context, viewName)
			require.NoError(t, err)
			require.NotEmpty(t, md.Triggers,
				"INSTEAD OF trigger must appear in per-table view inspect")
			tr := md.Triggers[0]
			require.Equal(t, trigName, tr.Name)
			require.Equal(t, "INSTEAD OF", tr.Timing)
			require.Contains(t, tr.Events, "INSERT")
		})
	}
}

// TestInspect_ViewDefinition_SQLServer verifies that view-typed tables carry
// a non-empty ViewDefinition loaded from sys.sql_modules (which does not
// truncate at 4000 chars unlike INFORMATION_SCHEMA.VIEWS.VIEW_DEFINITION).
// The view is dropped explicitly via DROP VIEW (not th.DropTable) before its
// base table is dropped.
func TestInspect_ViewDefinition_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			baseTbl := stringz.UniqTableName("vd_base")
			viewName := stringz.UniqTableName("vd_view")

			_, err := db.ExecContext(th.Context,
				`CREATE TABLE `+baseTbl+` (id INT NOT NULL PRIMARY KEY, val INT NOT NULL)`)
			require.NoError(t, err)
			// Register base-table cleanup FIRST so that LIFO ordering drops
			// the view (registered second) before the base table.
			t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })

			_, err = db.ExecContext(th.Context,
				`CREATE VIEW `+viewName+` AS SELECT id, val FROM `+baseTbl)
			require.NoError(t, err)
			// Register view cleanup SECOND (LIFO → runs before the base table).
			t.Cleanup(func() {
				_, _ = db.ExecContext(th.Context, `DROP VIEW `+viewName)
			})

			md, err := th.Open(src).TableMetadata(th.Context, viewName)
			require.NoError(t, err)
			require.NotEmpty(t, md.ViewDefinition,
				"view must have a non-empty ViewDefinition")
			require.Contains(t, md.ViewDefinition, baseTbl,
				"ViewDefinition should reference the base table name")
		})
	}
}

// TestForeignKey_CompositeOrdering_SQLServer verifies that a composite
// FK preserves the declared column pairing across (Columns, RefColumns).
// SQL Server's FK loader uses sys.foreign_key_columns ordered by
// constraint_column_id; a regression in that ORDER BY would scramble
// composite pairings. The parent PK uses (b, a) descending while the
// child FK uses (x, y) ascending so any loader bug that sorts either
// side independently — or pairs by name rather than by position — is
// caught.
func TestForeignKey_CompositeOrdering_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			parent := stringz.UniqTableName("fk_comp_parent")
			child := stringz.UniqTableName("fk_comp_child")
			_, err := db.ExecContext(th.Context,
				"CREATE TABLE "+parent+" (a INT NOT NULL, b INT NOT NULL, PRIMARY KEY (b, a))")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (x INT NOT NULL, y INT NOT NULL, "+
					"FOREIGN KEY (x, y) REFERENCES "+parent+" (b, a))")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.NotNil(t, md.FK)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Equal(t, parent, fk.RefTable)
			require.Equal(t, []string{"x", "y"}, fk.Columns)
			require.Equal(t, []string{"b", "a"}, fk.RefColumns)
		})
	}
}

// TestForeignKey_OnDeleteOnUpdate_SQLServer pins that the loader
// populates OnDelete / OnUpdate from sys.foreign_keys with the
// REPLACE-normalized form (no underscores). SQL Server reports
// "NO_ACTION", "SET_NULL" etc. via *_referential_action_desc; the
// REPLACE in getMSSQLForeignKeys rewrites them to the space-separated
// form used by the other drivers.
func TestForeignKey_OnDeleteOnUpdate_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			parent := stringz.UniqTableName("fk_act_parent")
			child := stringz.UniqTableName("fk_act_child")
			_, err := db.ExecContext(th.Context,
				"CREATE TABLE "+parent+" (id INT NOT NULL PRIMARY KEY)")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (parent_id INT, "+
					"FOREIGN KEY (parent_id) REFERENCES "+parent+
					" (id) ON DELETE CASCADE ON UPDATE SET NULL)")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Equal(t, "CASCADE", fk.OnDelete)
			require.Equal(t, "SET NULL", fk.OnUpdate,
				"loader must normalize SET_NULL → SET NULL to match the other drivers")
		})
	}
}

// TestForeignKey_SameCatalog_SQLServer pins the loader's current
// behavior for ordinary same-DB FKs: RefCatalog is left empty (the
// constraint is implicitly in the current catalog) and RefSchema is
// cleared by the loader's NULLIF when it matches the parent schema.
// SQL Server itself does not support declared cross-database FK
// constraints (only triggers/synonyms can fake them), so issue #616's
// original cross-catalog assertion reframes here as a same-catalog
// shape pin. A future loader change that surfaces cross-DB synonym
// refs should add a sibling test populating RefCatalog explicitly.
func TestForeignKey_SameCatalog_SQLServer(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	for _, handle := range sakila.MSAll() {
		t.Run(handle, func(t *testing.T) {
			t.Parallel()

			th := testh.New(t)
			src := th.Source(handle)
			db := th.OpenDB(src)

			parent := stringz.UniqTableName("fk_cat_parent")
			child := stringz.UniqTableName("fk_cat_child")
			_, err := db.ExecContext(th.Context,
				"CREATE TABLE "+parent+" (id INT NOT NULL PRIMARY KEY)")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (parent_id INT, "+
					"FOREIGN KEY (parent_id) REFERENCES "+parent+" (id))")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Empty(t, fk.RefCatalog,
				"same-DB FK must leave RefCatalog empty; populated only by a cross-catalog loader extension")
			require.Empty(t, fk.RefSchema,
				"same-schema FK must have RefSchema cleared by the loader's NULLIF")
		})
	}
}
