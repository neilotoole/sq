package markdownw_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/markdownw"
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

func TestMetadataWriter_SourceMetadata(t *testing.T) {
	const want = `# @test

| Property | Value |
| --- | --- |
| Name | testdb |
| Driver | sqlite3 |
| Schema | main |
| Size | 1.0MB |
| Tables | 2 |
| Views | 0 |

## Entity Relationship Diagram

` + "```mermaid" + `
erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
` + "```" + `

## Tables

### ` + "`actor`" + `

table · 200 rows

#### Entity Relationship Diagram

` + "```mermaid" + `
erDiagram
    actor {
        int actor_id PK
        text first_name
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
` + "```" + `

| Column | Type | Null | Key |
| --- | --- | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` | no | ` + "`PK`" + ` |
| ` + "`first_name`" + ` | ` + "`TEXT`" + ` | no |  |

**Foreign keys:**

| Direction | From | To | Constraint | On update | On delete |
| --- | --- | --- | --- | --- | --- |
| incoming | ` + "`film_actor(actor_id)`" +
		` | ` + "`actor(actor_id)`" +
		` | ` + "`fk_film_actor_actor`" +
		` | cascade | no action |

### ` + "`film_actor`" + `

table · 5462 rows

#### Entity Relationship Diagram

` + "```mermaid" + `
erDiagram
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
` + "```" + `

| Column | Type | Null | Key |
| --- | --- | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` | no | ` + "`PK,FK`" + ` |
| ` + "`film_id`" + ` | ` + "`INTEGER`" + ` | no | ` + "`PK`" + ` |

**Foreign keys:**

| Direction | From | To | Constraint | On update | On delete |
| --- | --- | --- | --- | --- | --- |
| outgoing | ` + "`film_actor(actor_id)`" +
		` | ` + "`actor(actor_id)`" +
		` | ` + "`fk_film_actor_actor`" +
		` | cascade | no action |
`

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), true))
	require.Equal(t, want, buf.String())
}

func TestMetadataWriter_SourceMetadata_overview(t *testing.T) {
	// Overview (showSchema=false): no ERD, no Tables, and table/view
	// counts are omitted since they aren't populated in overview mode.
	const want = `# @test

| Property | Value |
| --- | --- |
| Name | testdb |
| Driver | sqlite3 |
| Schema | main |
| Size | 1.0MB |
`

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), false))
	require.Equal(t, want, buf.String())
}

// TestMetadataWriter_indexesAndUniqueConstraints checks that indexes and
// unique constraints render as tables (the test source used elsewhere has
// neither). Type is empty for one index, exercising the blank cell.
func TestMetadataWriter_indexesAndUniqueConstraints(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t", TableType: "table", RowCount: 1,
		Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}},
		Indexes: []*metadata.Index{
			{Name: "t_pkey", Columns: []string{"id"}, Unique: true, Primary: true, Type: "BTREE"},
			{Name: "t_name_idx", Columns: []string{"name"}},
		},
		UniqueConstraints: []*metadata.UniqueConstraint{
			{Name: "t_email_key", Columns: []string{"email"}},
		},
	}

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.TableMetadata(tbl))
	out := buf.String()

	require.Contains(t, out, "**Indexes:**")
	require.Contains(t, out, "| Index | Columns | Unique | Primary | Type |")
	require.Contains(t, out, "| `t_pkey` | `id` | ✓ | ✓ | btree |")
	require.Contains(t, out, "| `t_name_idx` | `name` |  |  |  |")

	require.Contains(t, out, "**Unique constraints:**")
	require.Contains(t, out, "| Constraint | Columns |")
	require.Contains(t, out, "| `t_email_key` | `email` |")
}

func TestMetadataWriter_TableMetadata(t *testing.T) {
	const want = `# ` + "`film_actor`" + `

table · 5462 rows

## Entity Relationship Diagram

` + "```mermaid" + `
erDiagram
    film_actor {
        int actor_id PK,FK
        int film_id PK
    }
    actor ||--o{ film_actor : "fk_film_actor_actor"
` + "```" + `

| Column | Type | Null | Key |
| --- | --- | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` | no | ` + "`PK,FK`" + ` |
| ` + "`film_id`" + ` | ` + "`INTEGER`" + ` | no | ` + "`PK`" + ` |

**Foreign keys:**

| Direction | From | To | Constraint | On update | On delete |
| --- | --- | --- | --- | --- | --- |
| outgoing | ` + "`film_actor(actor_id)`" +
		` | ` + "`actor(actor_id)`" +
		` | ` + "`fk_film_actor_actor`" +
		` | cascade | no action |
`

	src := newTestSource()
	filmActor := src.Table("film_actor")
	require.NotNil(t, filmActor)

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.TableMetadata(filmActor))
	require.Equal(t, want, buf.String())
}

func TestMetadataWriter_Catalogs(t *testing.T) {
	const want = `| Catalog | Active |
| --- | :---: |
| sakila | ✓ |
| other |  |
`

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.Catalogs("sakila", []string{"sakila", "other"}))
	require.Equal(t, want, buf.String())
}

func TestMetadataWriter_Schemata(t *testing.T) {
	const want = `| Schema | Catalog | Owner | Active |
| --- | --- | --- | :---: |
| public | sakila | alice | ✓ |
| other | sakila |  |  |
`

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.Schemata("public", []*metadata.Schema{
		{Name: "public", Catalog: "sakila", Owner: "alice"},
		{Name: "other", Catalog: "sakila"},
	}))
	require.Equal(t, want, buf.String())
}

// TestMetadataWriter_empty verifies the no-op methods write nothing for
// empty inputs.
func TestMetadataWriter_empty(t *testing.T) {
	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())

	require.NoError(t, w.DBProperties(nil))
	require.NoError(t, w.DriverMetadata(nil))
	require.NoError(t, w.Catalogs("", nil))
	require.NoError(t, w.Schemata("", nil))
	require.Empty(t, buf.String())
}

// TestMetadataWriter_mermaidQuoting verifies that names which aren't
// bare-identifier-safe are handled correctly in the Mermaid diagram:
// entity (table) names are double-quoted (Mermaid supports that), but
// attribute (column) names and types are sanitized to underscores
// because Mermaid parses a quoted attribute token as a comment (which
// would be a syntax error). A column literally named "zip code" — as in
// the Postgres sakila customer_list view — must become zip_code.
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
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(src, true))

	got := buf.String()
	require.Contains(t, got, "```mermaid")
	// Entity name: quoted.
	require.Contains(t, got, `"weird table" {`)
	// Attribute names: sanitized, never quoted (a quoted attribute token
	// is a Mermaid comment and breaks the parse).
	require.Contains(t, got, "int id_col PK")
	require.Contains(t, got, "text zip_code")
	require.NotContains(t, got, `"id col"`)
	require.NotContains(t, got, `"zip code"`)
}
