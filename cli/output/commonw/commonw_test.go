package commonw_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestColumnKey(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		Columns: []*metadata.Column{
			{Name: "id", PrimaryKey: true},     // PK only
			{Name: "ref"},                      // FK only
			{Name: "uq"},                       // UK only
			{Name: "id_ref", PrimaryKey: true}, // PK and FK
			{Name: "plain"},                    // no key
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{
			{Columns: []string{"ref"}},
			{Columns: []string{"id_ref"}},
		}},
		UniqueConstraints: []*metadata.UniqueConstraint{{Columns: []string{"uq"}}},
	}
	fk := commonw.FKColumnSet(tbl)
	uc := commonw.UCColumnSet(tbl)

	require.Equal(t, "PK", commonw.ColumnKey(tbl.Columns[0], fk, uc))
	require.Equal(t, "FK", commonw.ColumnKey(tbl.Columns[1], fk, uc))
	require.Equal(t, "UK", commonw.ColumnKey(tbl.Columns[2], fk, uc))
	require.Equal(t, "PK,FK", commonw.ColumnKey(tbl.Columns[3], fk, uc))
	require.Equal(t, "", commonw.ColumnKey(tbl.Columns[4], fk, uc))
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

		// Outgoing rows precede incoming; actions are lower-cased.
		require.Equal(t, commonw.FKRow{
			Direction: "outgoing", From: "film(language_id)", To: "language(language_id)",
			Constraint: "film_language_id_fkey", OnUpdate: "cascade", OnDelete: "restrict",
		}, rows[0])

		// Incoming rows sort by constraint name (film_actor_ before inventory_).
		require.Equal(t, "incoming", rows[1].Direction)
		require.Equal(t, "film_actor_film_id_fkey", rows[1].Constraint)
		require.Equal(t, "film_actor(film_id)", rows[1].From)
		require.Equal(t, "film(film_id)", rows[1].To)
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
		require.Equal(t, "t(a, b)", rows[0].From)
		require.Equal(t, "cat.sch.other(x, y)", rows[0].To)
		require.Empty(t, rows[0].Constraint) // unnamed constraint
	})
}
