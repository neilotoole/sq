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

	// INCLUDE covering indexes were added in Postgres 11; there is no pre-11
	// equivalent to introspect.
	if !th.DBSemverAtLeast(sakila.Pg, "v11.0.0") {
		t.Skip("INCLUDE covering indexes require Postgres >= 11")
	}

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

	// Column flags are version-gated: identity columns arrived in Postgres 10,
	// stored generated columns in Postgres 12. Collation applies on every
	// supported version, so the base table always exercises it.
	hasIdentity := th.DBSemverAtLeast(sakila.Pg, "v10.0.0")
	hasGenerated := th.DBSemverAtLeast(sakila.Pg, "v12.0.0")

	tbl := stringz.UniqTableName("col_flags")
	_, err := db.ExecContext(th.Context,
		`CREATE TABLE `+tbl+` (
			first_name text,
			last_name  text,
			country    text COLLATE "C"
		)`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	if hasIdentity {
		_, err = db.ExecContext(th.Context,
			`ALTER TABLE `+tbl+` ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY`)
		require.NoError(t, err)
	}
	if hasGenerated {
		_, err = db.ExecContext(th.Context,
			`ALTER TABLE `+tbl+` ADD COLUMN full_name text `+
				`GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED`)
		require.NoError(t, err)
	}

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)

	countryCol := md.Column("country")
	require.NotNil(t, countryCol)
	require.Equal(t, "C", countryCol.Collation, "country: Collation should be 'C'")

	if hasIdentity {
		idCol := md.Column("id")
		require.NotNil(t, idCol)
		require.True(t, idCol.Identity, "id: Identity should be true")
	}
	if hasGenerated {
		fullNameCol := md.Column("full_name")
		require.NotNil(t, fullNameCol)
		require.True(t, fullNameCol.Generated, "full_name: Generated should be true")
		require.Contains(t, fullNameCol.GeneratedExpr, "first_name",
			"full_name: GeneratedExpr should contain 'first_name'")
	}
}

// TestPostgres_QuotedTableName_Metadata pins that table metadata loads for a
// table whose name contains a double-quote. getTableMetadata must resolve the
// OID via a pg_class JOIN on the bound name and use an escaped identifier in the
// COUNT subquery; raw interpolation built malformed SQL, which surfaced as a
// flake when a full-source scan happened to run while such a table existed
// (issue #1025).
func TestPostgres_QuotedTableName_Metadata(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	// The PRIMARY KEY matters: getPgConstraints resolves each constraint row's
	// table via quote_ident(kcu.table_name)::regclass, and that expression only
	// evaluates when the table has at least one constraint. Without the PK, the
	// constraint path would go untested for a quoted name.
	tbl := stringz.UniqTableName(`me"ta`)
	qtbl := stringz.DoubleQuote(tbl)
	_, err := db.ExecContext(th.Context, `CREATE TABLE `+qtbl+` (id INT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	_, err = db.ExecContext(th.Context, `INSERT INTO `+qtbl+` (id, name) VALUES (1, 'a'), (2, 'b')`)
	require.NoError(t, err)

	md, err := th.Open(src).TableMetadata(th.Context, tbl)
	require.NoError(t, err)
	require.Equal(t, tbl, md.Name)
	require.Equal(t, int64(2), md.RowCount, "RowCount must load for a quoted table name")
	require.Len(t, md.Columns, 2)

	idCol := md.Column("id")
	require.NotNil(t, idCol)
	require.True(t, idCol.PrimaryKey,
		"PK must load for a quoted table name (getPgConstraints)")
}

// TestPostgres_QuotedMatviewName_Metadata pins that matview metadata loads for
// a name that regclass text-parsing mishandles. getMatviewMetadata resolves
// name via quote_ident($1)::regclass; the previous raw $1::regclass parsed the
// value as SQL identifier syntax, so an uppercase name case-folded to a miss,
// a dotted name was read as a schema qualifier, and a leading double-quote was
// a syntax error. (An embedded mid-name double-quote happens to survive the
// old form, so it alone would not catch a regression.) The name here combines
// uppercase, a dot, and a double-quote to cover case-folding, qualification,
// and the COUNT(*) identifier escaping at once.
func TestPostgres_QuotedMatviewName_Metadata(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	// Not stringz.UniqTableName, which lowercases: the uppercase char is part
	// of the regression surface.
	mv := `Mv.we"ird__` + stringz.Uniq8()
	qmv := stringz.DoubleQuote(mv)
	_, err := db.ExecContext(th.Context,
		`CREATE MATERIALIZED VIEW `+qmv+` AS SELECT 1 AS id, 'a' AS label`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP MATERIALIZED VIEW IF EXISTS `+qmv)
	})

	md, err := th.Open(src).TableMetadata(th.Context, mv)
	require.NoError(t, err, "TableMetadata must load for a quoted matview name")
	require.Equal(t, mv, md.Name)
	require.Equal(t, sqlz.TableTypeMaterializedView, md.TableType)
	require.Equal(t, int64(1), md.RowCount, "RowCount must load for a quoted matview name")
	require.Len(t, md.Columns, 2)
	require.NotEmpty(t, md.ViewDefinition, "ViewDefinition must load for a quoted matview name")
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
			` FOR EACH ROW EXECUTE PROCEDURE `+fnName+`()`)
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

// TestPostgres_ViewInsteadOfTrigger verifies that an INSTEAD OF trigger on a
// view is returned by the per-table inspect path (Grip.TableMetadata), matching
// the behavior of source-wide inspect.
func TestPostgres_ViewInsteadOfTrigger(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	src := th.Source(sakila.Pg)
	db := th.OpenDB(src)

	baseTbl := stringz.UniqTableName("viot_base")
	viewName := stringz.UniqTableName("viot_view")
	fnName := viewName + "_fn"
	trigName := viewName + "_trig"

	// Register base-table cleanup FIRST (LIFO: view + trigger cleaned up first).
	_, err := db.ExecContext(th.Context, `CREATE TABLE `+baseTbl+` (id INT)`)
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })

	_, err = db.ExecContext(th.Context,
		`CREATE VIEW `+viewName+` AS SELECT id FROM `+baseTbl)
	require.NoError(t, err)
	// Register view cleanup SECOND (LIFO → runs before base-table cleanup).
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP VIEW IF EXISTS `+viewName)
	})

	_, err = db.ExecContext(th.Context,
		`CREATE FUNCTION `+fnName+`() RETURNS trigger LANGUAGE plpgsql AS $$`+
			` BEGIN RETURN NEW; END; $$`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, `DROP FUNCTION IF EXISTS `+fnName+`()`)
	})

	_, err = db.ExecContext(th.Context,
		`CREATE TRIGGER `+trigName+` INSTEAD OF INSERT ON `+viewName+
			` FOR EACH ROW EXECUTE PROCEDURE `+fnName+`()`)
	require.NoError(t, err)
	// Dropping the view removes its triggers; no separate trigger DROP needed.

	md, err := th.Open(src).TableMetadata(th.Context, viewName)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeView, md.TableType)
	require.NotEmpty(t, md.Triggers, "INSTEAD OF trigger must appear in per-table view inspect")

	var found *metadata.Trigger
	for _, tr := range md.Triggers {
		if tr.Name == trigName {
			found = tr
			break
		}
	}
	require.NotNil(t, found, "trigger %q not found in view Triggers", trigName)
	require.Equal(t, "INSTEAD OF", found.Timing)
	require.Contains(t, found.Events, "INSERT")
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
