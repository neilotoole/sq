package oracle_test

import (
	"strings"
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

// upperAll returns a copy of ss with each element uppercased. Oracle
// stores unquoted identifiers as upper case, so FK metadata round-trips
// in upper. Asserting against upper(want) keeps the literals in the
// test readable in their declared form.
func upperAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToUpper(s)
	}
	return out
}

// TestForeignKey_CompositeOrdering_Oracle verifies that a composite FK
// preserves the declared column pairing across (Columns, RefColumns).
// Oracle's FK loader (all_cons_columns joined via constraint_name +
// position) is the only common-driver loader where a missing ORDER BY
// would non-deterministically scramble composite FKs. The parent PK
// uses (b, a) descending while the child FK uses (x, y) ascending so
// any loader bug that sorts either side independently — or pairs by
// name rather than by position — is caught.
func TestForeignKey_CompositeOrdering_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	parent := stringz.UniqTableName("fk_comp_parent")
	child := stringz.UniqTableName("fk_comp_child")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+parent+" (a NUMBER NOT NULL, b NUMBER NOT NULL, PRIMARY KEY (b, a))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+child+" (x NUMBER NOT NULL, y NUMBER NOT NULL, "+
			"FOREIGN KEY (x, y) REFERENCES "+parent+" (b, a))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

	// Oracle stores unquoted identifiers upper-case; TableMetadata
	// looks them up by the stored form.
	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(child))
	require.NoError(t, err)
	require.NotNil(t, md.FK)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, strings.ToUpper(parent), fk.RefTable)
	require.Equal(t, upperAll([]string{"x", "y"}), fk.Columns)
	require.Equal(t, upperAll([]string{"b", "a"}), fk.RefColumns)
}

// TestForeignKey_OnDelete_Oracle pins that the loader populates
// OnDelete from all_constraints.delete_rule. Oracle exposes no
// equivalent for ON UPDATE referential actions, so OnUpdate must
// remain empty even when ON DELETE is set.
func TestForeignKey_OnDelete_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	parent := stringz.UniqTableName("fk_act_parent")
	child := stringz.UniqTableName("fk_act_child")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+parent+" (id NUMBER NOT NULL PRIMARY KEY)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+child+" (parent_id NUMBER, "+
			"FOREIGN KEY (parent_id) REFERENCES "+parent+" (id) ON DELETE CASCADE)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(child)) })

	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(child))
	require.NoError(t, err)
	require.Len(t, md.FK.Outgoing, 1)
	fk := md.FK.Outgoing[0]
	require.Equal(t, "CASCADE", fk.OnDelete)
	require.Empty(t, fk.OnUpdate,
		"Oracle exposes no ON UPDATE referential action; OnUpdate must stay empty")
}

// findColumn returns the named column (case-insensitive) from md, or fails.
func findColumn(t *testing.T, md *metadata.Table, name string) *metadata.Column {
	t.Helper()
	for _, c := range md.Columns {
		if strings.EqualFold(c.Name, name) {
			return c
		}
	}
	require.Failf(t, "column not found", "no column %q in table %q", name, md.Name)
	return nil
}

// TestInspectColumnFlags_Oracle pins the column-level flag and collation
// mapping sourced from USER_TAB_COLS: IDENTITY_COLUMN→Identity,
// VIRTUAL_COLUMN→Generated (+DATA_DEFAULT→GeneratedExpr), COLLATION→Collation.
// Oracle models auto-increment as identity, so AutoIncrement stays false.
func TestInspectColumnFlags_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("col_flags")
	// full_name is a virtual (generated) column; id is an identity column;
	// first_name is a plain VARCHAR2 carrying Oracle's default collation.
	// Explicit per-column COLLATE is rejected on MAX_STRING_SIZE=STANDARD
	// instances, so we assert the default collation Oracle reports instead.
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" ("+
			"id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, "+
			"first_name VARCHAR2(50), "+
			"last_name VARCHAR2(50), "+
			"full_name VARCHAR2(120) GENERATED ALWAYS AS (first_name || ' ' || last_name) VIRTUAL)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(tbl))
	require.NoError(t, err)

	id := findColumn(t, md, "id")
	require.True(t, id.Identity, "identity column must set Identity")
	require.False(t, id.AutoIncrement, "Oracle uses identity, not auto_increment")
	require.False(t, id.Generated, "identity column is not a generated/virtual column")

	full := findColumn(t, md, "full_name")
	require.True(t, full.Generated, "virtual column must set Generated")
	require.Contains(t, strings.ToUpper(full.GeneratedExpr), "FIRST_NAME",
		"GeneratedExpr should carry the virtual-column expression")

	first := findColumn(t, md, "first_name")
	require.False(t, first.Generated)
	require.False(t, first.Identity)
	require.NotEmpty(t, first.Collation,
		"a VARCHAR2 column should report Oracle's default collation")

	// Hidden/system columns (e.g. the identity sequence's backing column)
	// must not surface; HIDDEN_COLUMN='NO' filter keeps only the 4 declared.
	require.Len(t, md.Columns, 4, "only the declared, non-hidden columns")

	// Negative path: a plain base table has no view definition and no
	// triggers, so both must be empty (not errored). CheckConstraints is not
	// asserted here because Oracle auto-generates NOT NULL check constraints
	// whose filtering is covered by TestInspectCheckConstraint_Oracle.
	require.Empty(t, md.ViewDefinition, "base table must have no view definition")
	require.Empty(t, md.Triggers, "base table with no triggers must yield empty Triggers")
}

// TestInspectCheckConstraint_Oracle verifies real CHECK constraints are
// surfaced from USER_CONSTRAINTS while Oracle's auto-generated NOT NULL
// check constraints are filtered out.
func TestInspectCheckConstraint_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("chk")
	// age has a real CHECK; name is NOT NULL, which Oracle models as a
	// system-generated CHECK ("NAME" IS NOT NULL) that must NOT surface.
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" ("+
			"id NUMBER NOT NULL PRIMARY KEY, "+
			"name VARCHAR2(50) NOT NULL, "+
			"age NUMBER CONSTRAINT "+tbl+"_age_ck CHECK (age >= 0))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(tbl))
	require.NoError(t, err)

	require.Len(t, md.CheckConstraints, 1,
		"exactly one real CHECK; NOT NULL system checks must be filtered")
	cc := md.CheckConstraints[0]
	require.Equal(t, strings.ToUpper(tbl), cc.Table)
	require.Contains(t, strings.ToUpper(cc.Clause), "AGE >= 0")
	for _, c := range md.CheckConstraints {
		require.NotContains(t, strings.ToUpper(c.Clause), "IS NOT NULL",
			"NOT NULL system checks must be filtered out")
	}
}

// TestInspectTrigger_Oracle pins trigger metadata parsed from USER_TRIGGERS:
// TRIGGER_TYPE→Timing, TRIGGERING_EVENT→Events, STATUS→Enabled, and the
// best-effort TRIGGER_BODY→Definition (a LONG column).
func TestInspectTrigger_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("trg_tbl")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (id NUMBER NOT NULL PRIMARY KEY, n NUMBER)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	trg := strings.ToUpper(tbl) + "_TRG"
	_, err = db.ExecContext(th.Context,
		"CREATE OR REPLACE TRIGGER "+trg+" BEFORE INSERT OR UPDATE ON "+tbl+
			" FOR EACH ROW BEGIN NULL; END;")
	require.NoError(t, err)
	// The trigger drops with the table; no separate cleanup needed.

	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(tbl))
	require.NoError(t, err)

	require.Len(t, md.Triggers, 1)
	tr := md.Triggers[0]
	require.Equal(t, "BEFORE", tr.Timing)
	require.Equal(t, []string{"INSERT", "UPDATE"}, tr.Events)
	require.NotNil(t, tr.Enabled)
	require.True(t, *tr.Enabled)
	require.NotEmpty(t, tr.Definition, "TRIGGER_BODY (LONG) should be readable")
}

// TestInspectTriggerUpdateOf_Oracle verifies that a column-scoped update trigger
// ("BEFORE UPDATE OF col ...") produces Events containing plain "UPDATE" (not
// "UPDATE OF col"). On Oracle 23c USER_TRIGGERS.TRIGGERING_EVENT already strips
// the column qualifier and stores "INSERT OR UPDATE", so the normalization in
// getOracleTriggers is a defensive no-op here; the test acts as a regression
// guard for any Oracle version that might retain the qualifier.
func TestInspectTriggerUpdateOf_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("trg_upd_of")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (id NUMBER NOT NULL PRIMARY KEY, n NUMBER)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	// Declare a column-scoped update trigger plus INSERT.
	// Oracle 23c stores TRIGGERING_EVENT as "INSERT OR UPDATE" (column qualifier
	// stripped); the normalization code handles any version that retains it.
	trg := strings.ToUpper(tbl) + "_TRG"
	_, err = db.ExecContext(th.Context,
		"CREATE OR REPLACE TRIGGER "+trg+
			" BEFORE UPDATE OF n OR INSERT ON "+tbl+
			" FOR EACH ROW BEGIN NULL; END;")
	require.NoError(t, err)
	// The trigger drops with the table; no separate cleanup needed.

	md, err := th.Open(src).TableMetadata(th.Context, strings.ToUpper(tbl))
	require.NoError(t, err)

	require.Len(t, md.Triggers, 1)
	tr := md.Triggers[0]
	require.Equal(t, "BEFORE", tr.Timing)
	// Events must be normalized: "UPDATE OF N" → "UPDATE", not the raw form.
	require.ElementsMatch(t, []string{"UPDATE", "INSERT"}, tr.Events)
}

// TestInspectViewInsteadOfTrigger_Oracle verifies that an INSTEAD OF trigger
// on a view is returned by the per-table inspect path (Grip.TableMetadata),
// matching the behavior of source-wide inspect.
func TestInspectViewInsteadOfTrigger_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	baseTbl := strings.ToUpper(stringz.UniqTableName("viot_base"))
	viewName := strings.ToUpper(stringz.UniqTableName("viot_view"))
	trigName := viewName + "_TRG"

	// Register base-table cleanup FIRST (LIFO: view cleaned up before table).
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+baseTbl+" (id NUMBER NOT NULL PRIMARY KEY)")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(baseTbl)) })

	_, err = db.ExecContext(th.Context,
		"CREATE VIEW "+viewName+" AS SELECT id FROM "+baseTbl)
	require.NoError(t, err)
	// Register view cleanup SECOND (LIFO → runs before base-table cleanup).
	// Dropping the view automatically drops its INSTEAD OF triggers in Oracle,
	// but we drop explicitly for clarity.
	t.Cleanup(func() {
		_, _ = db.ExecContext(th.Context, "DROP VIEW "+viewName)
	})

	_, err = db.ExecContext(th.Context,
		"CREATE TRIGGER "+trigName+" INSTEAD OF INSERT ON "+viewName+
			" FOR EACH ROW BEGIN NULL; END;")
	require.NoError(t, err)
	// INSTEAD OF triggers are dropped with the view; no separate DROP needed.

	md, err := th.Open(src).TableMetadata(th.Context, viewName)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeView, md.TableType)
	require.NotEmpty(t, md.Triggers,
		"INSTEAD OF trigger must appear in per-table view inspect")

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

// TestInspectViewDefinition_Oracle pins that a view's defining SQL
// (USER_VIEWS.TEXT, a LONG column) is captured into ViewDefinition.
func TestInspectViewDefinition_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("vdef_base")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (id NUMBER NOT NULL PRIMARY KEY, name VARCHAR2(50))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	vw := strings.ToUpper(tbl) + "_V"
	_, err = db.ExecContext(th.Context,
		"CREATE VIEW "+vw+" AS SELECT id, name FROM "+tbl+" WHERE id > 0")
	require.NoError(t, err)
	// View cleanup registered AFTER base-table cleanup so LIFO drops the
	// view first (th.DropTable does not drop views).
	t.Cleanup(func() { _, _ = db.ExecContext(th.Context, "DROP VIEW "+vw) })

	md, err := th.Open(src).TableMetadata(th.Context, vw)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeView, md.TableType)
	require.NotEmpty(t, md.ViewDefinition, "USER_VIEWS.TEXT (LONG) should be readable")
	require.Contains(t, strings.ToUpper(md.ViewDefinition), "SELECT")
}

// TestInspectMaterializedViewType_Oracle pins that a materialized view is
// classified with the canonical TableTypeMaterializedView (not Table),
// retains DBTableType "MATERIALIZED VIEW", and carries its defining query
// (USER_MVIEWS.QUERY, a LONG column) in ViewDefinition.
func TestInspectMaterializedViewType_Oracle(t *testing.T) {
	tu.SkipShort(t, true)
	t.Parallel()

	th := testh.New(t)
	if !th.SourceConfigured(sakila.Ora) {
		t.Skip("Oracle Sakila source not configured")
	}
	src := th.Source(sakila.Ora)
	db := th.OpenDB(src)

	tbl := stringz.UniqTableName("mv_base")
	_, err := db.ExecContext(th.Context,
		"CREATE TABLE "+tbl+" (id NUMBER NOT NULL PRIMARY KEY, name VARCHAR2(50))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(tbl)) })

	mv := strings.ToUpper(tbl) + "_MV"
	_, err = db.ExecContext(th.Context,
		"CREATE MATERIALIZED VIEW "+mv+" AS SELECT id, name FROM "+tbl)
	require.NoError(t, err)
	// Matview cleanup registered AFTER base-table cleanup so LIFO drops the
	// matview first (th.DropTable does not drop materialized views).
	t.Cleanup(func() { _, _ = db.ExecContext(th.Context, "DROP MATERIALIZED VIEW "+mv) })

	md, err := th.Open(src).TableMetadata(th.Context, mv)
	require.NoError(t, err)
	require.Equal(t, sqlz.TableTypeMaterializedView, md.TableType,
		"materialized view must use the canonical materialized_view type")
	require.Equal(t, "MATERIALIZED VIEW", md.DBTableType)
	require.NotEmpty(t, md.ViewDefinition, "USER_MVIEWS.QUERY (LONG) should be readable")
}
