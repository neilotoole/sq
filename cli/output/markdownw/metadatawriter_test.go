package markdownw_test

import (
	"bytes"
	"testing"
	"time"

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

	var size int64 = 1048576
	src := &metadata.Source{
		Handle:     "@test",
		Name:       "testdb",
		Driver:     drivertype.Type("sqlite3"),
		Schema:     "main",
		Size:       &size,
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
| Name | ` + "`testdb`" + ` |
| Driver | ` + "`sqlite3`" + ` |
| Schema | ` + "`main`" + ` |
| Size | ` + "`1.0MB`" + ` |
| Tables | ` + "`2`" + ` |
| Views | ` + "`0`" + ` |

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

[` + "`actor`" + `](#actor) · [` + "`film_actor`" + `](#film_actor)

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

| Column | Type | Nullable | PK | FK |
| --- | --- | :---: | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` |  | ✓ |  |
| ` + "`first_name`" + ` | ` + "`TEXT`" + ` |  |  |  |

**Foreign keys:**

| Relationship (→ references · ← referenced by) | Constraint | On update | On delete |
| --- | --- | --- | --- |
| ` + "`actor_id`" + ` ← ` +
		"`film_actor.actor_id`" +
		` | ` + "`fk_film_actor_actor`" + ` | cascade |  |

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

| Column | Type | Nullable | PK | FK |
| --- | --- | :---: | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` |  | ✓ | ✓ |
| ` + "`film_id`" + ` | ` + "`INTEGER`" + ` |  | ✓ |  |

**Foreign keys:**

| Relationship (→ references · ← referenced by) | Constraint | On update | On delete |
| --- | --- | --- | --- |
| ` + "`actor_id`" + ` → ` +
		"`actor.actor_id`" +
		` | ` + "`fk_film_actor_actor`" + ` | cascade |  |
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
| Name | ` + "`testdb`" + ` |
| Driver | ` + "`sqlite3`" + ` |
| Schema | ` + "`main`" + ` |
| Size | ` + "`1.0MB`" + ` |
`

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(newTestSource(), false))
	require.Equal(t, want, buf.String())
}

// TestMetadataWriter_SourceMetadata_nilSize verifies that a source whose
// driver doesn't report a size renders "-" rather than "0.0B" (gh744).
func TestMetadataWriter_SourceMetadata_nilSize(t *testing.T) {
	src := newTestSource()
	src.Size = nil

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(src, true))

	got := buf.String()
	// The Size key-value row is emitted with a literal "-".
	require.Contains(t, got, "| Size | `-` |")
	require.NotContains(t, got, "0.0B")
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

// TestMetadataWriter_backtickIdentifier checks that an identifier containing a
// backtick renders as a valid widened-fence code span rather than a broken
// single-backtick span.
func TestMetadataWriter_backtickIdentifier(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t", TableType: "table", RowCount: 1,
		Columns: []*metadata.Column{{Name: "a`b", ColumnType: "int"}},
	}

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.TableMetadata(tbl))
	out := buf.String()

	// The fence widens to two backticks so the embedded backtick is literal
	// rather than closing the span early.
	require.Contains(t, out, "``a`b``")
}

// TestMetadataWriter_views checks that a source with views renders the
// "Tables & views" heading and italicizes view links in the TOC (tables
// stay plain).
func TestMetadataWriter_views(t *testing.T) {
	var size int64 = 1024
	src := &metadata.Source{
		Handle: "@test", Name: "db", Driver: drivertype.Type("sqlite3"),
		Schema: "main", Size: &size, TableCount: 1, ViewCount: 1,
		Tables: []*metadata.Table{
			{Name: "t_actor", TableType: "table", Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}}},
			{Name: "v_films", TableType: "view", Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}}},
		},
	}

	buf := &bytes.Buffer{}
	w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
	require.NoError(t, w.SourceMetadata(src, true))
	out := buf.String()

	require.Contains(t, out, "## Tables & views")
	require.Contains(t, out, "[`t_actor`](#t_actor)")      // table link: plain
	require.Contains(t, out, "*[`v_films`](#v_films)*")    // view link: italicized
	require.NotContains(t, out, "*[`t_actor`](#t_actor)*") // table not italicized
}

// TestMetadataWriter_provenance checks that the "generated by" line appears
// under the title (for both source and single-table docs) when Printing
// carries generation info, and is omitted otherwise (keeping goldens stable).
func TestMetadataWriter_provenance(t *testing.T) {
	pr := output.NewPrinting()
	pr.GeneratedAt = time.Date(2026, 5, 22, 13, 14, 15, 0, time.UTC)
	// "sq inspect" (linked to its docs) @ the timestamp, both as code.
	const want = "*[`sq inspect`](https://sq.io/docs/inspect) @ `2026-05-22T13:14:15Z`*"

	srcBuf := &bytes.Buffer{}
	require.NoError(t, markdownw.NewMetadataWriter(srcBuf, pr).SourceMetadata(newTestSource(), true))
	require.Contains(t, srcBuf.String(), want)

	tblBuf := &bytes.Buffer{}
	require.NoError(t, markdownw.NewMetadataWriter(tblBuf, pr).TableMetadata(newTestSource().Tables[0]))
	require.Contains(t, tblBuf.String(), want)

	// Omitted when no generation info is set (the default in other tests).
	// Assert on the docs URL, which only the provenance line emits.
	plainBuf := &bytes.Buffer{}
	require.NoError(t, markdownw.NewMetadataWriter(plainBuf, output.NewPrinting()).SourceMetadata(newTestSource(), true))
	require.NotContains(t, plainBuf.String(), "sq.io/docs/inspect")
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

| Column | Type | Nullable | PK | FK |
| --- | --- | :---: | :---: | :---: |
| ` + "`actor_id`" + ` | ` + "`INTEGER`" + ` |  | ✓ | ✓ |
| ` + "`film_id`" + ` | ` + "`INTEGER`" + ` |  | ✓ |  |

**Foreign keys:**

| Relationship (→ references · ← referenced by) | Constraint | On update | On delete |
| --- | --- | --- | --- |
| ` + "`actor_id`" + ` → ` +
		"`actor.actor_id`" +
		` | ` + "`fk_film_actor_actor`" + ` | cascade |  |
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

// TestMetadataWriter_InspectEnrichment verifies that the new inspect-metadata
// fields (Identity/AutoIncrement/Generated, GeneratedExpr, Collation,
// CheckConstraints, Triggers, ViewDefinition) render correctly. No DB is
// required: the fixture is hand-constructed.
func TestMetadataWriter_InspectEnrichment(t *testing.T) {
	enabled := true

	richTable := &metadata.Table{
		Name:      "orders",
		TableType: "table",
		RowCount:  1000,
		Columns: []*metadata.Column{
			{
				Name:       "id",
				Position:   1,
				PrimaryKey: true,
				ColumnType: "BIGINT",
				Kind:       kind.Int,
				Identity:   true,
			},
			{
				Name:          "total",
				Position:      2,
				ColumnType:    "NUMERIC",
				Kind:          kind.Decimal,
				Generated:     true,
				GeneratedExpr: "qty * price",
			},
			{
				Name:       "note",
				Position:   3,
				ColumnType: "TEXT",
				Kind:       kind.Text,
				Collation:  "en_US.utf8",
			},
			{
				Name:          "qty",
				Position:      4,
				ColumnType:    "INT",
				Kind:          kind.Int,
				AutoIncrement: true,
			},
		},
		CheckConstraints: []*metadata.CheckConstraint{
			{Name: "orders_total_pos", Table: "orders", Clause: "total >= 0"},
			{Name: "orders_qty_pos", Table: "orders", Clause: "qty > 0"},
		},
		Triggers: []*metadata.Trigger{
			{
				Name:    "trg_orders_audit",
				Table:   "orders",
				Timing:  "AFTER",
				Events:  []string{"INSERT", "UPDATE"},
				Enabled: &enabled,
				Definition: "CREATE TRIGGER trg_orders_audit AFTER INSERT OR UPDATE ON orders" +
					" FOR EACH ROW EXECUTE FUNCTION audit_log();",
			},
		},
	}

	viewTable := &metadata.Table{
		Name:           "v_order_summary",
		TableType:      "view",
		RowCount:       0,
		ViewDefinition: "SELECT id, total FROM orders WHERE total > 0",
		Columns: []*metadata.Column{
			{Name: "id", Position: 1, ColumnType: "BIGINT", Kind: kind.Int},
			{Name: "total", Position: 2, ColumnType: "NUMERIC", Kind: kind.Decimal},
		},
	}

	t.Run("rich_table", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
		require.NoError(t, w.TableMetadata(richTable))
		out := buf.String()

		// Auto column: header and values for all three auto types.
		require.Contains(t, out, "| Auto |", "Auto column header")
		require.Contains(t, out, "identity", "identity auto label")
		require.Contains(t, out, "generated", "generated auto label")
		require.Contains(t, out, "auto_inc", "auto_inc auto label")

		// Generated Expr column.
		require.Contains(t, out, "| Generated Expr |", "Generated Expr column header")
		require.Contains(t, out, "qty * price", "generated expr value")

		// Collation column.
		require.Contains(t, out, "| Collation |", "Collation column header")
		require.Contains(t, out, "en_US.utf8", "collation value")

		// Check constraints section.
		require.Contains(t, out, "**Check constraints:**", "check constraints heading")
		require.Contains(t, out, "orders_total_pos", "check constraint name")
		require.Contains(t, out, "total &gt;= 0", "check clause (HTML-escaped '>')")
		require.Contains(t, out, "orders_qty_pos", "second check constraint name")

		// Triggers section.
		require.Contains(t, out, "**Triggers:**", "triggers heading")
		require.Contains(t, out, "trg_orders_audit", "trigger name")
		require.Contains(t, out, "AFTER", "trigger timing")
		require.Contains(t, out, "INSERT, UPDATE", "trigger events")
		require.Contains(t, out, "✓", "trigger enabled mark")
		require.Contains(t, out, "trg_orders_audit AFTER INSERT OR UPDATE", "trigger definition snippet")
	})

	t.Run("view_definition", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
		require.NoError(t, w.TableMetadata(viewTable))
		out := buf.String()

		require.Contains(t, out, "**View definition:**", "view definition heading")
		require.Contains(t, out, "```sql", "sql fenced block")
		require.Contains(t, out, "SELECT id, total FROM orders WHERE total > 0", "view definition body")
	})

	t.Run("plain_table_unchanged", func(t *testing.T) {
		// A plain table with none of the new fields should NOT render
		// any new sections or columns.
		plain := &metadata.Table{
			Name: "plain", TableType: "table", RowCount: 1,
			Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}},
		}
		buf := &bytes.Buffer{}
		w := markdownw.NewMetadataWriter(buf, output.NewPrinting())
		require.NoError(t, w.TableMetadata(plain))
		out := buf.String()

		require.NotContains(t, out, "| Auto |", "no Auto column for plain table")
		require.NotContains(t, out, "| Generated Expr |", "no Generated Expr for plain table")
		require.NotContains(t, out, "| Collation |", "no Collation for plain table")
		require.NotContains(t, out, "**Check constraints:**", "no check constraints for plain table")
		require.NotContains(t, out, "**Triggers:**", "no triggers for plain table")
		require.NotContains(t, out, "**View definition:**", "no view definition for plain table")
	})
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
