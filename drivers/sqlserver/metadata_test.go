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

// TestForeignKey_CompositeOrdering_SQLServer verifies that a composite
// FK preserves the declared column pairing across (Columns, RefColumns).
// SQL Server's FK loader uses sys.foreign_key_columns ordered by
// constraint_column_id; a regression in that ORDER BY would scramble
// composite pairings.
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
				"CREATE TABLE "+parent+" (a INT NOT NULL, b INT NOT NULL, PRIMARY KEY (a, b))")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
			_, err = db.ExecContext(th.Context,
				"CREATE TABLE "+child+" (x INT NOT NULL, y INT NOT NULL, "+
					"FOREIGN KEY (x, y) REFERENCES "+parent+" (a, b))")
			require.NoError(t, err)
			t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

			md, err := th.Open(src).TableMetadata(th.Context, child)
			require.NoError(t, err)
			require.NotNil(t, md.FK)
			require.Len(t, md.FK.Outgoing, 1)
			fk := md.FK.Outgoing[0]
			require.Equal(t, parent, fk.RefTable)
			require.Equal(t, []string{"x", "y"}, fk.Columns)
			require.Equal(t, []string{"a", "b"}, fk.RefColumns)
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
				"REPLACE(update_referential_action_desc, '_', ' ') must rewrite SET_NULL to SET NULL")
		})
	}
}

// TestForeignKey_SameCatalog_SQLServer pins the loader's current
// behavior for ordinary same-DB FKs: RefCatalog is left empty (the
// constraint is implicitly in the current catalog) and RefSchema is
// cleared by the loader's NULLIF when it matches the parent schema.
// SQL Server itself does not support declared cross-database FK
// constraints (only triggers/synonyms can fake them), so the issue
// #616 P3 row reframes here as "pin the same-catalog shape and
// document the cross-catalog gap". A future loader change that
// surfaces cross-DB synonym refs should add a sibling test populating
// RefCatalog explicitly.
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
