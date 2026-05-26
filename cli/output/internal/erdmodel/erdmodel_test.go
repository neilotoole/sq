package erdmodel_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/internal/erdmodel"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// childFK builds a "child" table with a single foreign key on column "ref_id"
// pointing at "parent". The column's nullability, whether it's the primary
// key, and any unique constraint covering it are controlled by the args, so
// each cardinality branch of Resolve can be exercised in isolation.
func childFK(nullable, isPK, uniqueConstraint bool) *metadata.Table {
	child := &metadata.Table{
		Name: "child", TableType: "table",
		Columns: []*metadata.Column{
			{Name: "ref_id", Position: 1, PrimaryKey: isPK, Nullable: nullable, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk_child_parent", Table: "child", Columns: []string{"ref_id"},
			RefTable: "parent", RefColumns: []string{"id"},
		}}},
	}
	if uniqueConstraint {
		child.UniqueConstraints = []*metadata.UniqueConstraint{
			{Name: "uq_child_ref", Table: "child", Columns: []string{"ref_id"}},
		}
	}
	return child
}

func TestResolve(t *testing.T) {
	testCases := []struct {
		name          string
		nullable      bool
		isPK          bool
		uniqueCons    bool
		wantParentOpt bool
		wantChildUniq bool
	}{
		{name: "one_to_many", wantParentOpt: false, wantChildUniq: false},
		{name: "nullable_makes_parent_optional", nullable: true, wantParentOpt: true},
		{name: "pk_makes_child_unique", isPK: true, wantChildUniq: true},
		{name: "unique_constraint_makes_child_unique", uniqueCons: true, wantChildUniq: true},
		{name: "nullable_and_unique", nullable: true, uniqueCons: true, wantParentOpt: true, wantChildUniq: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			child := childFK(tc.nullable, tc.isPK, tc.uniqueCons)
			byName := map[string]*metadata.Table{child.Name: child}

			edge, ok := erdmodel.Resolve(child.FK.Outgoing[0], byName)
			require.True(t, ok)
			require.Equal(t, "parent", edge.Parent)
			require.Equal(t, "child", edge.Child)
			require.Equal(t, "fk_child_parent", edge.Label)
			require.Equal(t, tc.wantParentOpt, edge.Card.ParentOptional)
			require.Equal(t, tc.wantChildUniq, edge.Card.ChildUnique)
		})
	}
}

// TestResolve_skipped verifies the references that Resolve declines to draw:
// a nil FK, and cross-source references (non-empty RefCatalog/RefSchema, set
// by metadata.LinkForeignKeys for targets outside this source).
func TestResolve_skipped(t *testing.T) {
	_, ok := erdmodel.Resolve(nil, nil)
	require.False(t, ok, "nil FK is skipped")

	crossSchema := &metadata.ForeignKey{
		Table: "child", Columns: []string{"ref_id"},
		RefSchema: "other", RefTable: "parent", RefColumns: []string{"id"},
	}
	_, ok = erdmodel.Resolve(crossSchema, nil)
	require.False(t, ok, "cross-schema reference is skipped")

	crossCatalog := &metadata.ForeignKey{
		Table: "child", Columns: []string{"ref_id"},
		RefCatalog: "other", RefTable: "parent", RefColumns: []string{"id"},
	}
	_, ok = erdmodel.Resolve(crossCatalog, nil)
	require.False(t, ok, "cross-catalog reference is skipped")
}

// TestResolve_unknownChild verifies that when the child (referencing) table
// isn't available for inference, cardinality defaults to exactly-one to
// zero-or-many (both flags false).
func TestResolve_unknownChild(t *testing.T) {
	fk := &metadata.ForeignKey{
		Name: "fk_child_parent", Table: "child", Columns: []string{"ref_id"},
		RefTable: "parent", RefColumns: []string{"id"},
	}
	edge, ok := erdmodel.Resolve(fk, map[string]*metadata.Table{}) // child absent
	require.True(t, ok)
	require.False(t, edge.Card.ParentOptional)
	require.False(t, edge.Card.ChildUnique)
}

func TestKeyMarker(t *testing.T) {
	fkCols := map[string]bool{"ref_id": true}
	require.Equal(t, "PK", erdmodel.KeyMarker(&metadata.Column{Name: "id", PrimaryKey: true}, fkCols))
	require.Equal(t, "FK", erdmodel.KeyMarker(&metadata.Column{Name: "ref_id"}, fkCols))
	require.Equal(t, "PK,FK", erdmodel.KeyMarker(&metadata.Column{Name: "ref_id", PrimaryKey: true}, fkCols))
	require.Empty(t, erdmodel.KeyMarker(&metadata.Column{Name: "other"}, fkCols))
}
