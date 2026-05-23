package htmlw_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/htmlw"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

func TestMetadataWriter_Catalogs(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.Catalogs("sakila", []string{"sakila", "other"}))

	got := buf.String()
	require.Contains(t, got, "<!doctype html>")
	require.Contains(t, got, "<table>")
	require.Contains(t, got, "<td><code>sakila</code></td>")
	require.NotContains(t, got, `class="mermaid"`)
}

func TestMetadataWriter_Schemata(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.Schemata("public", []*metadata.Schema{
		{Name: "public", Catalog: "sakila", Owner: "alice"},
	}))
	require.Contains(t, buf.String(), "<code>public</code>")
}

func TestMetadataWriter_escaping(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.Catalogs("", []string{"a<b>&c"}))
	got := buf.String()
	require.Contains(t, got, "a&lt;b&gt;&amp;c")
	require.NotContains(t, got, "a<b>&c")
}

func newTestSource() *metadata.Source {
	actor := &metadata.Table{
		Name: "actor", TableType: "table", RowCount: 200,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER"},
			{Name: "first_name", Position: 2, ColumnType: "TEXT"},
		},
	}
	filmActor := &metadata.Table{
		Name: "film_actor", TableType: "table", RowCount: 5462,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER"},
			{Name: "film_id", Position: 2, PrimaryKey: true, ColumnType: "INTEGER"},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk_film_actor_actor", Table: "film_actor", Columns: []string{"actor_id"},
			RefTable: "actor", RefColumns: []string{"actor_id"},
		}}},
	}
	src := &metadata.Source{
		Handle: "@test", Name: "testdb", Schema: "main", Size: 1048576,
		TableCount: 2, ViewCount: 0, Tables: []*metadata.Table{actor, filmActor},
	}
	metadata.LinkForeignKeys(nil, src)
	return src
}

func TestMetadataWriter_SourceMetadata(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(newTestSource(), true))

	got := buf.String()
	require.Contains(t, got, "<!doctype html>")
	require.Contains(t, got, "<title>@test</title>")
	require.Contains(t, got, "<h1>@test</h1>")
	require.Contains(t, got, `<h2 id="tables" class="sq-tables">`)
	// A table-of-contents under the Tables heading links to each section.
	require.Contains(t, got, `<nav class="sq-toc">`)
	require.Contains(t, got, `<a href="#actor"><code>actor</code></a>`)
	// Per-table headings are prominent (sq-table class) and deep-linkable
	// (id + self-link), e.g. #actor.
	require.Contains(t, got, `<h3 id="actor" class="sq-table">`)
	require.Contains(t, got, `<a class="sq-anchor" href="#actor"><code>actor</code></a>`)
	require.Contains(t, got, ".sq-table {")
	require.Contains(t, got, `<pre class="mermaid">`)
	// The ER-diagram headings are deep-linkable too: #erd for the
	// whole-source diagram, #<table>-erd for each per-table diagram.
	require.Contains(t, got, `<h2 id="erd">`)
	require.Contains(t, got, `id="actor-erd"`)
	require.Contains(t, got, "actor ||--o{ film_actor")
	require.Contains(t, got, "<td><code>actor_id</code></td>")
	// The Key column is split into PK / FK / Unique ✓ columns.
	require.Contains(t, got, "<th>PK</th>")
	require.Contains(t, got, "<th>FK</th>")
	require.Contains(t, got, "cdn.jsdelivr.net/npm/mermaid@11")
	// The diagram renders on a light panel so the default-theme tables and
	// connector lines stay legible even when the page is in dark mode.
	require.Contains(t, got, "pre.mermaid { background: #fff;")
	// Diagrams are click-to-zoom: panzoom is loaded (CDN here) and the
	// overlay module + cursor affordance are present.
	require.Contains(t, got, "panzoom@9/+esm")
	require.Contains(t, got, "sq-erd-overlay")
	require.Contains(t, got, "cursor: zoom-in")
	// Foreign keys render as a single anchored "Relationship" column; the
	// test source has an outgoing FK (film_actor → actor) and its matching
	// incoming back-reference (on actor), distinguished by colored arrows.
	// The section label is a deep-linkable <caption>.
	require.Contains(t, got, `<caption id="actor-columns">`)
	require.Contains(t, got, `<caption id="film_actor-foreign-keys">`)
	require.Contains(t, got, `<a class="sq-anchor" href="#actor-columns">Columns</a>`)
	require.Contains(t, got,
		`<th>Relationship <span class="fk-legend">(<span class="fk-out">→</span> references · `+
			`<span class="fk-in">←</span> referenced by)</span></th>`)
	// Outgoing: this table's column → referenced; incoming: ← referencing.
	require.Contains(t, got,
		`<code>film_actor.actor_id</code> <span class="fk-out">→</span> <code>actor.actor_id</code>`)
	require.Contains(t, got,
		`<code>actor.actor_id</code> <span class="fk-in">←</span> <code>film_actor.actor_id</code>`)
}

func TestMetadataWriter_SourceMetadata_overview(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(newTestSource(), false))
	got := buf.String()
	require.NotContains(t, got, `class="mermaid"`)
	require.NotContains(t, got, `id="tables"`)
}

// TestMetadataWriter_indexesAndUniqueConstraints checks that indexes and
// unique constraints render as <table> elements with the expected columns.
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
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.TableMetadata(tbl))
	got := buf.String()

	require.Contains(t, got, `<caption id="t-indexes">`)
	require.Contains(t, got, "<th>Index</th>")
	require.Contains(t, got, "<th>Unique</th>")
	require.Contains(t, got, "<td><code>t_pkey</code></td>")
	require.Contains(t, got, "<td>✓</td>")
	require.Contains(t, got, "<td>btree</td>")
	require.Contains(t, got, `<caption id="t-unique-constraints">`)
	require.Contains(t, got, "<th>Constraint</th>")
	require.Contains(t, got, "<td><code>t_email_key</code></td>")
}

// TestMetadataWriter_views checks the "Tables & views" heading and the
// tinted view-chip class (sq-view) in the TOC; table chips stay plain.
func TestMetadataWriter_views(t *testing.T) {
	src := &metadata.Source{
		Handle: "@test", Name: "db", Driver: drivertype.Type("sqlite3"),
		Schema: "main", Size: 1024, TableCount: 1, ViewCount: 1,
		Tables: []*metadata.Table{
			{Name: "t_actor", TableType: "table", Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}}},
			{Name: "v_films", TableType: "view", Columns: []*metadata.Column{{Name: "id", ColumnType: "int"}}},
		},
	}

	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(src, true))
	got := buf.String()

	require.Contains(t, got, "Tables &amp; views")                  // heading, & escaped
	require.Contains(t, got, ".sq-toc a.sq-view code")              // tint rule present
	require.Contains(t, got, `<a class="sq-view" href="#v_films">`) // view chip tinted
	require.Contains(t, got, `<a href="#t_actor">`)                 // table chip plain
}

func TestMetadataWriter_embed(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), true)
	require.NoError(t, w.SourceMetadata(newTestSource(), true))
	got := buf.String()
	require.Greater(t, len(got), 500_000, "embedded output inlines the mermaid library")
	require.NotContains(t, got, "cdn.jsdelivr.net")
	require.Contains(t, got, "mermaid.initialize")
	// The click-to-zoom overlay works offline too: panzoom is vendored
	// (no CDN import) and the overlay module is present.
	require.Contains(t, got, "sq-erd-overlay")
	require.NotContains(t, got, "panzoom@9/+esm")
}
