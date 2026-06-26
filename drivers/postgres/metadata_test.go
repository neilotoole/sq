package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/sqlz"
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

// TestPostgres_CheckConstraints verifies that CHECK constraints are
// populated from the Postgres catalog via pg_constraint.
func TestPostgres_CheckConstraints(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("chk_widget")
	conName := tbl + "_price_positive"
	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (id INT, price NUMERIC, CONSTRAINT `+conName+` CHECK (price > 0))`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.NotEmpty(t, md.CheckConstraints, "CheckConstraints should not be empty")

	var found bool
	for _, cc := range md.CheckConstraints {
		if cc.Name == conName {
			found = true
			require.Contains(t, cc.Clause, "price",
				"CHECK clause should contain 'price'")
			break
		}
	}
	require.True(t, found, "CHECK constraint %q not found in CheckConstraints", conName)
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

// TestPostgres_ViewDefinition verifies that ViewDefinition is populated for
// views and is empty for base tables.
func TestPostgres_ViewDefinition(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("vdef_base")
	view := stringz.UniqTableName("vdef_view")

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (id INT, price NUMERIC)`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		`CREATE VIEW `+view+` AS SELECT id, price FROM `+tbl+` WHERE price > 0`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP VIEW IF EXISTS `+view)
		th.DropTable(src, tablefq.From(tbl))
	})

	viewMd, err := th.Open(src).TableMetadata(th.Context, view)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeView, viewMd.TableType, "TableType should be 'view'")
	require.NotEmpty(t, viewMd.ViewDefinition, "ViewDefinition should not be empty for a view")
	require.Contains(t, viewMd.ViewDefinition, "WHERE",
		"ViewDefinition should contain 'WHERE'")

	// Base table must have empty ViewDefinition.
	tblMd, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.Empty(t, tblMd.ViewDefinition, "ViewDefinition should be empty for a base table")
}

// TestPostgres_Triggers verifies that trigger metadata is populated from the
// Postgres catalog via pg_trigger.
func TestPostgres_Triggers(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("trig_widget")
	fnName := tbl + "_fn"
	trigName := tbl + "_audit"

	_, err := db.ExecContext(th.Context, `CREATE TABLE `+tbl+` (id INT)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		th.DropTable(src, tablefq.From(tbl))
		_, _ = db.ExecContext(th.Context, `DROP FUNCTION IF EXISTS `+fnName+`()`)
	})

	_, err = db.ExecContext(th.Context,
		`CREATE FUNCTION `+fnName+`() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END; $$`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		`CREATE TRIGGER `+trigName+` AFTER INSERT OR UPDATE ON `+tbl+
			` FOR EACH ROW EXECUTE FUNCTION `+fnName+`()`)
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.NotEmpty(t, md.Triggers, "Triggers should not be empty")

	var found *metadata.Trigger
	for _, tr := range md.Triggers {
		if tr.Name == trigName {
			found = tr
			break
		}
	}
	require.NotNil(t, found, "trigger %q not found in Triggers", trigName)
	require.Equal(t, "AFTER", found.Timing)
	require.Contains(t, found.Events, "INSERT")
	require.Contains(t, found.Events, "UPDATE")
	require.NotNil(t, found.Enabled)
	require.True(t, *found.Enabled)
	require.Contains(t, found.Definition, "CREATE TRIGGER")
}

// TestPostgres_Matview verifies that materialized views (which live only in
// pg_catalog, not information_schema) are surfaced by sq inspect via both the
// per-table and source-wide metadata paths, with real columns, view
// definition, row count, and indexes.
func TestPostgres_Matview(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("mv_base")
	mv := stringz.UniqTableName("mv_view")
	ix := stringz.UniqTableName("mv_idx")

	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (id INT, label TEXT)`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		`INSERT INTO `+tbl+` (id, label) VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma')`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		`CREATE MATERIALIZED VIEW `+mv+` AS SELECT id, label FROM `+tbl+` WHERE id > 0`)
	require.NoError(t, err)

	_, err = db.ExecContext(th.Context,
		`CREATE INDEX `+ix+` ON `+mv+` (id)`)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP MATERIALIZED VIEW IF EXISTS `+mv)
		th.DropTable(src, tablefq.From(tbl))
	})

	// Per-table path.
	mvMd, err := th.Open(src).TableMetadata(th.Context, mv)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeMaterializedView, mvMd.TableType,
		"TableType should be materialized_view")
	require.Equal(t, "MATERIALIZED VIEW", mvMd.DBTableType)
	require.NotEmpty(t, mvMd.Columns, "matview columns must be populated")
	require.NotNil(t, mvMd.Column("id"))
	require.NotNil(t, mvMd.Column("label"))
	require.NotEmpty(t, mvMd.ViewDefinition, "ViewDefinition should not be empty")
	require.Contains(t, mvMd.ViewDefinition, "label",
		"ViewDefinition should contain a recognizable column fragment")
	require.NotNil(t, pgIndexByName(mvMd.Indexes, ix),
		"index %q should appear in matview indexes", ix)
	require.Equal(t, int64(3), mvMd.RowCount, "RowCount should match inserted rows")

	// Source-wide path.
	srcMd, err := th.Open(src).SourceMetadata(th.Context, false)
	require.NoError(t, err)

	var mvFromSrc *metadata.Table
	for _, tm := range srcMd.Tables {
		if tm.Name == mv {
			mvFromSrc = tm
			break
		}
	}
	require.NotNil(t, mvFromSrc, "matview %q should be present in SourceMetadata", mv)
	require.Equal(t, sqlz.TableTypeMaterializedView, mvFromSrc.TableType)
	require.NotEmpty(t, mvFromSrc.Columns, "matview columns must be populated in source-wide path")
	require.NotNil(t, pgIndexByName(mvFromSrc.Indexes, ix),
		"index %q should be attached in source-wide path", ix)
}
