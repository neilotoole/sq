package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
	"github.com/neilotoole/sq/testh/tu"
)

func pgIndexByName(idxs []*metadata.Index, name string) *metadata.Index {
	for _, idx := range idxs {
		if idx.Name == name {
			return idx
		}
	}
	return nil
}

// TestIndexes_ExpressionArity_Postgres verifies that a Postgres
// expression key (attnum=0 in pg_index.indkey) is preserved as an
// empty-string sentinel, and that an all-expression index is omitted.
func TestIndexes_ExpressionArity_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("idx_arity")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (a INT, b TEXT, c INT)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_mixed ON "+tbl+" (a, lower(b), c)")
	require.NoError(t, err)
	_, err = db.ExecContext(th.Context,
		"CREATE INDEX ix_allexpr ON "+tbl+" (lower(b))")
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	mixed := pgIndexByName(md.Indexes, "ix_mixed")
	require.NotNil(t, mixed, "ix_mixed should be reported")
	require.Equal(t, []string{"a", "", "c"}, mixed.Columns,
		"the lower(b) key position must be the empty-string sentinel")
	require.Nil(t, pgIndexByName(md.Indexes, "ix_allexpr"),
		"an all-expression index must be omitted")
}

// TestIndexes_IncludeFilter_Postgres pins that INCLUDE/covering columns
// are excluded from Index.Columns (only key columns appear).
func TestIndexes_IncludeFilter_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("idx_include")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (k INT, extra TEXT)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE UNIQUE INDEX ix_inc ON "+tbl+" (k) INCLUDE (extra)")
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	inc := pgIndexByName(md.Indexes, "ix_inc")
	require.NotNil(t, inc)
	require.Equal(t, []string{"k"}, inc.Columns,
		"INCLUDE column 'extra' must not appear in Index.Columns")
}

// TestForeignKey_CompositeOrdering_Postgres verifies that a composite FK
// preserves the declared column pairing across (Columns, RefColumns) —
// a driver miscounting ordinal positions in the row scan would scramble
// the pairing and ship a structurally invalid FK.
//
// The parent PK is declared (b, a) — alphabetically descending — while
// the child FK is declared (x, y) — alphabetically ascending. This
// asymmetry means a loader that pairs by position correctly produces
// Columns=[x,y] / RefColumns=[b,a], but any loader bug that sorts
// either side independently (or that pairs by name rather than by
// position) yields a deterministically wrong result that this test
// catches.
func TestForeignKey_CompositeOrdering_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	parent := stringz.UniqTableName("fk_comp_parent")
	child := stringz.UniqTableName("fk_comp_child")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+parent+" (a INT, b INT, PRIMARY KEY (b, a))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+child+" (x INT, y INT, FOREIGN KEY (x, y) REFERENCES "+parent+" (b, a))")
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
}

// TestForeignKey_OnDeleteOnUpdate_Postgres pins that the loader
// populates OnDelete / OnUpdate from
// information_schema.referential_constraints — sakila itself uses
// default actions, so this exercises explicit non-default values.
func TestForeignKey_OnDeleteOnUpdate_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	parent := stringz.UniqTableName("fk_act_parent")
	child := stringz.UniqTableName("fk_act_child")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+parent+" (id INT PRIMARY KEY)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+child+" (parent_id INT REFERENCES "+parent+
			" (id) ON DELETE CASCADE ON UPDATE SET NULL)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

	md, err := th.Open(src).TableMetadata(th.Context, child)
	require.NoError(t, err)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, "CASCADE", fk.OnDelete)
	require.Equal(t, "SET NULL", fk.OnUpdate)
}

// TestForeignKey_ReservedWordColumn_Postgres verifies that a quoted
// reserved-word column name ("from") round-trips through the FK loader
// unquoted in Columns / RefColumns. A loader that strips or
// double-quotes the identifier would surface the wrong name.
func TestForeignKey_ReservedWordColumn_Postgres(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	parent := stringz.UniqTableName("fk_rw_parent")
	child := stringz.UniqTableName("fk_rw_child")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+parent+" (id INT PRIMARY KEY)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		`CREATE TABLE `+child+` ("from" INT, FOREIGN KEY ("from") REFERENCES `+parent+` (id))`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

	md, err := th.Open(src).TableMetadata(th.Context, child)
	require.NoError(t, err)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, []string{"from"}, fk.Columns,
		`quoted reserved-word column "from" must round-trip unquoted in Columns`)
	require.Equal(t, []string{"id"}, fk.RefColumns)
}

// TestPostgres_ColumnFlags verifies that identity, generated, and collation
// column metadata fields are correctly populated from the Postgres catalog.
func TestPostgres_ColumnFlags(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("col_flags")
	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (
			id         bigint GENERATED ALWAYS AS IDENTITY,
			first_name text,
			last_name  text,
			full_name  text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED,
			country    text COLLATE "C"
		)`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	idCol := md.Column("id")
	require.NotNil(t, idCol)
	require.True(t, idCol.Identity, "id: Identity should be true")

	fullNameCol := md.Column("full_name")
	require.NotNil(t, fullNameCol)
	require.True(t, fullNameCol.Generated, "full_name: Generated should be true")
	require.Contains(t, fullNameCol.GeneratedExpr, "first_name",
		"full_name: GeneratedExpr should contain 'first_name'")

	countryCol := md.Column("country")
	require.NotNil(t, countryCol)
	require.Equal(t, "C", countryCol.Collation, "country: Collation should be 'C'")
}
