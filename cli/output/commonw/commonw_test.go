package commonw_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestFKColumnSet(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{
			{Columns: []string{"ref"}},
			{Columns: []string{"id_ref"}},
		}},
	}

	fk := commonw.FKColumnSet(tbl)
	require.True(t, fk["ref"])
	require.True(t, fk["id_ref"])
	require.False(t, fk["other"])
}

func TestViews(t *testing.T) {
	tbl := &metadata.Table{Name: "t", TableType: "table"}
	view := &metadata.Table{Name: "v", TableType: "view"}
	matview := &metadata.Table{Name: "mv", TableType: sqlz.TableTypeMaterializedView}

	require.False(t, commonw.IsView(tbl))
	require.True(t, commonw.IsView(view))
	require.True(t, commonw.IsView(matview))
	require.False(t, commonw.IsView(nil))

	require.False(t, commonw.HasViews([]*metadata.Table{tbl}))
	require.True(t, commonw.HasViews([]*metadata.Table{tbl, view}))
	require.True(t, commonw.HasViews([]*metadata.Table{tbl, matview}))
}

func TestFKRows(t *testing.T) {
	t.Run("nil_and_empty", func(t *testing.T) {
		require.Nil(t, commonw.FKRows(nil))
		require.Nil(t, commonw.FKRows(&metadata.Table{Name: "t"}))
		require.Nil(t, commonw.FKRows(&metadata.Table{Name: "t", FK: &metadata.FKGroup{}}))
	})

	t.Run("outgoing_then_incoming", func(t *testing.T) {
		tbl := &metadata.Table{
			Name: "film",
			FK: &metadata.FKGroup{
				Outgoing: []*metadata.ForeignKey{{
					Name: "film_language_id_fkey", Table: "film",
					Columns:  []string{"language_id"},
					RefTable: "language", RefColumns: []string{"language_id"},
					OnUpdate: "CASCADE", OnDelete: "RESTRICT",
				}},
				Incoming: []*metadata.ForeignKey{
					{
						Name: "inventory_film_id_fkey", Table: "inventory",
						Columns:  []string{"film_id"},
						RefTable: "film", RefColumns: []string{"film_id"},
					},
					{
						Name: "film_actor_film_id_fkey", Table: "film_actor",
						Columns:  []string{"film_id"},
						RefTable: "film", RefColumns: []string{"film_id"},
					},
				},
			},
		}

		rows := commonw.FKRows(tbl)
		require.Len(t, rows, 3)

		// Outgoing rows precede incoming; actions are lower-cased. The Local
		// side is the bare owning column; Remote keeps its table qualifier.
		require.Equal(t, commonw.FKRow{
			Direction: "outgoing", Local: "language_id", Remote: "language.language_id",
			Constraint: "film_language_id_fkey", OnUpdate: "cascade", OnDelete: "restrict",
		}, rows[0])

		// Incoming rows sort by constraint name (film_actor_ before inventory_).
		require.Equal(t, "incoming", rows[1].Direction)
		require.Equal(t, "film_actor_film_id_fkey", rows[1].Constraint)
		require.Equal(t, "film_id", rows[1].Local)
		require.Equal(t, "film_actor.film_id", rows[1].Remote)
		require.Equal(t, "inventory_film_id_fkey", rows[2].Constraint)
	})

	t.Run("composite_and_cross_source", func(t *testing.T) {
		tbl := &metadata.Table{
			Name: "t",
			FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
				Table: "t", Columns: []string{"a", "b"},
				RefCatalog: "cat", RefSchema: "sch", RefTable: "other",
				RefColumns: []string{"x", "y"},
			}}},
		}
		rows := commonw.FKRows(tbl)
		require.Len(t, rows, 1)
		require.Equal(t, "(a, b)", rows[0].Local) // composite local → bare paren form
		require.Equal(t, "cat.sch.other(x, y)", rows[0].Remote)
		require.Empty(t, rows[0].Constraint) // unnamed constraint
	})

	t.Run("no_action_omitted", func(t *testing.T) {
		tbl := &metadata.Table{
			Name: "t",
			FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
				Table: "t", Columns: []string{"a"},
				RefTable: "u", RefColumns: []string{"id"},
				OnUpdate: "NO ACTION", OnDelete: "SET NULL",
			}}},
		}
		rows := commonw.FKRows(tbl)
		require.Len(t, rows, 1)
		require.Empty(t, rows[0].OnUpdate, `"NO ACTION" is omitted`)
		require.Equal(t, "set null", rows[0].OnDelete)
	})
}

func TestIndexRows(t *testing.T) {
	require.Nil(t, commonw.IndexRows(nil))
	require.Nil(t, commonw.IndexRows(&metadata.Table{Name: "t"}))

	tbl := &metadata.Table{
		Name: "film",
		Indexes: []*metadata.Index{
			{Name: "idx_title", Columns: []string{"title"}, Type: "BTREE"},
			{Name: "film_pkey", Columns: []string{"film_id"}, Unique: true, Primary: true, Type: "BTREE"},
			{Name: "film_uq", Columns: []string{"a", "b"}, Unique: true},
		},
	}

	rows := commonw.IndexRows(tbl)
	require.Len(t, rows, 3)
	// Sorted by name; Type is lower-cased; composite columns joined.
	require.Equal(t, commonw.IndexRow{
		Name: "film_pkey", Columns: "film_id", Unique: true, Primary: true, Type: "btree",
	}, rows[0])
	require.Equal(t, commonw.IndexRow{
		Name: "film_uq", Columns: "a, b", Unique: true,
	}, rows[1])
	require.Equal(t, commonw.IndexRow{
		Name: "idx_title", Columns: "title", Type: "btree",
	}, rows[2])
}

func TestIndexRows_OmitsExpressionSentinel(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		Indexes: []*metadata.Index{
			{Name: "ix_mixed", Columns: []string{"a", "", "c"}},
		},
	}

	rows := commonw.IndexRows(tbl)
	require.Len(t, rows, 1)
	require.Equal(t, "a, c", rows[0].Columns,
		"expression-key sentinels must be omitted from the joined column string")
}

func TestUCRows(t *testing.T) {
	require.Nil(t, commonw.UCRows(nil))
	require.Nil(t, commonw.UCRows(&metadata.Table{Name: "t"}))

	tbl := &metadata.Table{
		Name: "t",
		UniqueConstraints: []*metadata.UniqueConstraint{
			{Name: "t_email_key", Columns: []string{"email"}},
			{Name: "", Columns: []string{"x", "y"}}, // unnamed
		},
	}

	rows := commonw.UCRows(tbl)
	require.Len(t, rows, 2)
	// Sorted by name; the unnamed constraint (empty name) sorts first.
	require.Equal(t, commonw.UCRow{Name: "", Columns: "x, y"}, rows[0])
	require.Equal(t, commonw.UCRow{Name: "t_email_key", Columns: "email"}, rows[1])
}
