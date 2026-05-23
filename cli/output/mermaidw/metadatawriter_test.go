package mermaidw_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/mermaidw"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// newTestSource builds a small deterministic two-table source
// (actor + film_actor, with film_actor.actor_id → actor.actor_id) and
// links its foreign keys so FK.Incoming is populated. It returns a
// fresh *metadata.Source on each call so subtests don't share state.
func newTestSource() *metadata.Source {
	actor := &metadata.Table{
		Name:      "actor",
		TableType: "table",
		RowCount:  200,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "first_name", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}
	filmActor := &metadata.Table{
		Name:      "film_actor",
		TableType: "table",
		RowCount:  5462,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "film_id", Position: 2, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{
			Outgoing: []*metadata.ForeignKey{
				{
					Name:       "fk_film_actor_actor",
					Table:      "film_actor",
					Columns:    []string{"actor_id"},
					RefTable:   "actor",
					RefColumns: []string{"actor_id"},
					OnDelete:   "NO ACTION",
					OnUpdate:   "CASCADE",
				},
			},
		},
	}

	src := &metadata.Source{
		Handle:     "@test",
		Name:       "testdb",
		Driver:     drivertype.Type("sqlite3"),
		Schema:     "main",
		Size:       1048576,
		TableCount: 2,
		ViewCount:  0,
		Tables:     []*metadata.Table{actor, filmActor},
	}
	metadata.LinkForeignKeys(nil, src)
	return src
}

// TestMetadataWriter_SourceMetadata checks the whole-source ERD: bare
// erDiagram source with no Markdown fence or HTML wrapper, entities in a
// deterministic order (tables before views, then by name).
func TestMetadataWriter_SourceMetadata(t *testing.T) {
	const want = `erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`

	buf := &bytes.Buffer{}
	w := mermaidw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), true))
	require.Equal(t, want, buf.String())
}

// TestMetadataWriter_TableMetadata checks the focused single-table ERD.
func TestMetadataWriter_TableMetadata(t *testing.T) {
	const want = `erDiagram
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
`

	src := newTestSource()
	filmActor := src.Table("film_actor")
	require.NotNil(t, filmActor)

	buf := &bytes.Buffer{}
	w := mermaidw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.TableMetadata(filmActor))
	require.Equal(t, want, buf.String())
}

// TestMetadataWriter_unsupported verifies that operations with no ERD
// representation return an error (and write nothing), rather than emitting
// confusing empty output.
func TestMetadataWriter_unsupported(t *testing.T) {
	buf := &bytes.Buffer{}
	w := mermaidw.NewMetadataWriter(buf, output.NewPrinting())

	// Overview mode (showSchema=false) carries no schema to diagram.
	require.ErrorContains(t, w.SourceMetadata(newTestSource(), false), "mermaid-erd")
	require.ErrorContains(t, w.DBProperties(map[string]any{"k": "v"}), "mermaid-erd")
	require.ErrorContains(t, w.DriverMetadata(nil), "mermaid-erd")
	require.ErrorContains(t, w.Catalogs("sakila", []string{"sakila"}), "mermaid-erd")
	require.ErrorContains(t, w.Schemata("public", []*metadata.Schema{{Name: "public"}}), "mermaid-erd")
	require.Empty(t, buf.String())
}

// TestMetadataWriter_emptyDiagram verifies that a source/table with nothing
// to diagram (no columns, no foreign keys) returns an error and writes
// nothing, rather than silently emitting empty output.
func TestMetadataWriter_emptyDiagram(t *testing.T) {
	buf := &bytes.Buffer{}
	w := mermaidw.NewMetadataWriter(buf, output.NewPrinting())

	// Source with a single column-less table: nothing to render.
	src := &metadata.Source{
		Handle: "@empty", Name: "empty", Driver: drivertype.Type("sqlite3"),
		Tables: []*metadata.Table{{Name: "t", TableType: "table"}},
	}
	require.ErrorContains(t, w.SourceMetadata(src, true), "mermaid-erd")
	require.Empty(t, buf.String())

	// Source with no tables at all.
	buf.Reset()
	require.ErrorContains(t, w.SourceMetadata(&metadata.Source{Handle: "@e"}, true), "mermaid-erd")
	require.Empty(t, buf.String())

	// Single column-less table.
	buf.Reset()
	require.ErrorContains(t, w.TableMetadata(&metadata.Table{Name: "t", TableType: "table"}), "mermaid-erd")
	require.Empty(t, buf.String())
}

// TestMetadataWriter_mermaidQuoting verifies that names which aren't
// bare-identifier-safe are handled the same way the markdown/html writers
// handle them: entity (table) names are double-quoted, but attribute
// (column) names and types are sanitized to underscores (a quoted attribute
// token is parsed by Mermaid as a comment).
func TestMetadataWriter_mermaidQuoting(t *testing.T) {
	src := &metadata.Source{
		Handle: "@q",
		Name:   "q",
		Driver: drivertype.Type("sqlite3"),
		Tables: []*metadata.Table{
			{
				Name:      "weird table",
				TableType: "table",
				Columns: []*metadata.Column{
					{Name: "id col", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
					{Name: "zip code", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
				},
			},
		},
	}

	buf := &bytes.Buffer{}
	w := mermaidw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(src, true))

	got := buf.String()
	require.Contains(t, got, "erDiagram")
	require.Contains(t, got, `"weird table" {`)
	require.Contains(t, got, "int id_col PK")
	require.Contains(t, got, "text zip_code")
	require.NotContains(t, got, `"id col"`)
	require.NotContains(t, got, `"zip code"`)
}
