package oracle_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
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
// would non-deterministically scramble composite FKs.
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
		"CREATE TABLE "+parent+" (a NUMBER NOT NULL, b NUMBER NOT NULL, PRIMARY KEY (a, b))")
	require.NoError(t, err)
	t.Cleanup(func() { th.DropTable(src, tablefq.From(parent)) })
	_, err = db.ExecContext(th.Context,
		"CREATE TABLE "+child+" (x NUMBER NOT NULL, y NUMBER NOT NULL, "+
			"FOREIGN KEY (x, y) REFERENCES "+parent+" (a, b))")
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
	require.Equal(t, upperAll([]string{"a", "b"}), fk.RefColumns)
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
