package mermaid_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func testTables() []*metadata.Table {
	actor := &metadata.Table{
		Name: "actor", TableType: "table", RowCount: 200,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "first_name", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}
	filmActor := &metadata.Table{
		Name: "film_actor", TableType: "table", RowCount: 5462,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "film_id", Position: 2, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk_film_actor_actor", Table: "film_actor", Columns: []string{"actor_id"},
			RefTable: "actor", RefColumns: []string{"actor_id"},
		}}},
	}
	src := &metadata.Source{Handle: "@test", Tables: []*metadata.Table{actor, filmActor}}
	metadata.LinkForeignKeys(nil, src)
	return src.Tables
}

func TestSourceDiagram(t *testing.T) {
	got := mermaid.SourceDiagram(testTables())
	require.Equal(t, `erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`, got)
}

func TestTableDiagram_focused(t *testing.T) {
	tables := testTables()
	got := mermaid.TableDiagram(tables[1], mermaid.Index(tables)) // film_actor
	require.Equal(t, `erDiagram
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`, got)
}

// TestTableDiagram_nilCardIndex tests the single-table inspect path where
// cardIndex is nil. Neighbor cardinality falls back to the default "||--o{".
// The actor table has an incoming FK from film_actor (established via
// LinkForeignKeys), so the edge should appear in the output.
func TestTableDiagram_nilCardIndex(t *testing.T) {
	tables := testTables()
	actor := tables[0]
	got := mermaid.TableDiagram(actor, nil)
	require.Equal(t, `erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`, got)
}

// TestTableDiagram_incomingFK_withIndex tests the incoming-FK branch with a
// populated cardIndex. The actor table receives an FK from film_actor;
// film_actor.actor_id is part of a composite PK so the cardinality is still
// "||--o{" (not one-to-one).
func TestTableDiagram_incomingFK_withIndex(t *testing.T) {
	tables := testTables()
	actor := tables[0]
	got := mermaid.TableDiagram(actor, mermaid.Index(tables))
	require.Equal(t, `erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`, got)
}

func TestSourceDiagram_quoting(t *testing.T) {
	src := &metadata.Source{Handle: "@q", Tables: []*metadata.Table{{
		Name: "weird table", TableType: "table",
		Columns: []*metadata.Column{
			{Name: "id col", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "zip code", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}}}
	got := mermaid.SourceDiagram(src.Tables)
	require.Equal(t, `erDiagram
    "weird table" {
        int id_col PK
        text zip_code
    }
`, got)
}

func TestSourceDiagram_empty(t *testing.T) {
	require.Equal(t, "", mermaid.SourceDiagram(nil))
}
