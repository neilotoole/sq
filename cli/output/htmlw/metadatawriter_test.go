package htmlw_test

import (
	"bytes"
	"testing"
	"time"

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
	var size int64 = 1048576
	src := &metadata.Source{
		Handle: "@test", Name: "testdb", Schema: "main", Size: &size,
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
	// Outgoing: this table's bare column → referenced; incoming: ← referencing.
	// The owning-table prefix is dropped on the left (it's implied here).
	require.Contains(t, got,
		`<code>actor_id</code> <span class="fk-out">→</span> <code>actor.actor_id</code>`)
	require.Contains(t, got,
		`<code>actor_id</code> <span class="fk-in">←</span> <code>film_actor.actor_id</code>`)
}

func TestMetadataWriter_SourceMetadata_overview(t *testing.T) {
	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(newTestSource(), false))
	got := buf.String()
	require.NotContains(t, got, `class="mermaid"`)
	require.NotContains(t, got, `id="tables"`)
}

// TestMetadataWriter_SourceMetadata_nilSize verifies that a source whose
// driver doesn't report a size renders "-" in the Size row rather than
// "0.0B" (gh744). The row is still emitted so the key-value layout stays
// consistent across sources.
func TestMetadataWriter_SourceMetadata_nilSize(t *testing.T) {
	src := newTestSource()
	src.Size = nil

	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(src, true))

	got := buf.String()
	// The Size key-value row is rendered with a literal "-" value.
	require.Contains(t, got, "<tr><td>Size</td><td><code>-</code></td></tr>")
	require.NotContains(t, got, "0.0B")
}

// TestMetadataWriter_DBSemver checks that the "DB semver" row appears when
// Source.DBSemver is set, and is omitted when it's empty (addRow skips
// empty values).
func TestMetadataWriter_DBSemver(t *testing.T) {
	src := newTestSource()
	src.DBVersion = "3.45.1"
	src.DBSemver = "v3.45.1"

	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(src, true))
	got := buf.String()
	require.Contains(t, got, "DB semver")
	require.Contains(t, got, "v3.45.1")

	src.DBSemver = ""
	buf2 := &bytes.Buffer{}
	w2 := htmlw.NewMetadataWriter(buf2, output.NewPrinting(), false)
	require.NoError(t, w2.SourceMetadata(src, true))
	got2 := buf2.String()
	require.NotContains(t, got2, "DB semver")
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
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.SourceMetadata(src, true))
	got := buf.String()

	require.Contains(t, got, "Tables &amp; views")                  // heading, & escaped
	require.Contains(t, got, ".sq-toc a.sq-view code")              // tint rule present
	require.Contains(t, got, `<a class="sq-view" href="#v_films">`) // view chip tinted
	require.Contains(t, got, `<a href="#t_actor">`)                 // table chip plain
}

// TestMetadataWriter_provenance checks the muted "generated by" line appears
// under the title (source and single-table docs) when Printing carries
// generation info, and is omitted otherwise (keeping other tests stable).
func TestMetadataWriter_provenance(t *testing.T) {
	pr := output.NewPrinting()
	pr.GeneratedAt = time.Date(2026, 5, 22, 13, 14, 15, 0, time.UTC)
	// "sq inspect" (linked to its docs, <a> inside <code>) @ the timestamp.
	const want = `<p class="sq-gen"><code>` +
		`<a href="https://sq.io/docs/inspect">sq inspect</a></code> ` +
		`@ <code>2026-05-22T13:14:15Z</code></p>`

	srcBuf := &bytes.Buffer{}
	require.NoError(t, htmlw.NewMetadataWriter(srcBuf, pr, false).SourceMetadata(newTestSource(), true))
	require.Contains(t, srcBuf.String(), want)
	require.Contains(t, srcBuf.String(), ".sq-gen {")         // style rule present
	require.Contains(t, srcBuf.String(), ".sq-gen a:hover {") // underline on hover only
	require.Contains(t, srcBuf.String(), ".sq-gen a { color: #2563eb;")

	tblBuf := &bytes.Buffer{}
	require.NoError(t, htmlw.NewMetadataWriter(tblBuf, pr, false).TableMetadata(newTestSource().Tables[0]))
	require.Contains(t, tblBuf.String(), want)

	// Omitted when no generation info is set. Assert on the docs URL, which
	// only the provenance link emits (the .sq-gen CSS rules are always present).
	plainBuf := &bytes.Buffer{}
	plainW := htmlw.NewMetadataWriter(plainBuf, output.NewPrinting(), false)
	require.NoError(t, plainW.SourceMetadata(newTestSource(), true))
	require.NotContains(t, plainBuf.String(), "sq.io/docs/inspect")
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

// TestMetadataWriter_InspectEnrichment verifies that the HTML writer renders
// the new inspect metadata: Auto/GeneratedExpr/Collation columns in the
// column table, check-constraint section, trigger section, and view-definition
// block, each conditional on data presence.
func TestMetadataWriter_InspectEnrichment(t *testing.T) {
	enabled := true

	enriched := &metadata.Table{
		Name:      "enriched",
		TableType: "table",
		RowCount:  42,
		Columns: []*metadata.Column{
			{Name: "id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Identity: true},
			{Name: "name", Position: 2, ColumnType: "TEXT", Collation: "utf8_bin"},
			{Name: "full_name", Position: 3, ColumnType: "TEXT", Generated: true, GeneratedExpr: "first || last"},
		},
		CheckConstraints: []*metadata.CheckConstraint{
			{Name: "chk_name_len", Table: "enriched", Clause: "length(name) > 0"},
		},
		Triggers: []*metadata.Trigger{
			{
				Name: "trg_audit", Table: "enriched",
				Timing: "AFTER", Events: []string{"INSERT", "UPDATE"},
				Enabled:    &enabled,
				Definition: "CREATE TRIGGER trg_audit ...",
			},
		},
	}

	buf := &bytes.Buffer{}
	w := htmlw.NewMetadataWriter(buf, output.NewPrinting(), false)
	require.NoError(t, w.TableMetadata(enriched))
	got := buf.String()

	// Auto column present and labels correct.
	require.Contains(t, got, "<th>Auto</th>")
	require.Contains(t, got, "identity")  // id column
	require.Contains(t, got, "generated") // full_name column

	// Generated Expr column present with SQL expression.
	require.Contains(t, got, "<th>Generated Expr</th>")
	require.Contains(t, got, "<code>first || last</code>")

	// Collation column present.
	require.Contains(t, got, "<th>Collation</th>")
	require.Contains(t, got, "utf8_bin")

	// Check constraints table with Clause HTML-escaped.
	require.Contains(t, got, "Check constraints")
	require.Contains(t, got, "<th>Constraint</th>")
	require.Contains(t, got, "<th>Clause</th>")
	require.Contains(t, got, "<code>chk_name_len</code>")
	require.Contains(t, got, "length(name) &gt; 0") // '>' is HTML-escaped

	// Triggers table with conditional Enabled and Definition columns.
	require.Contains(t, got, "Triggers")
	require.Contains(t, got, "<th>Trigger</th>")
	require.Contains(t, got, "<th>Timing</th>")
	require.Contains(t, got, "<th>Events</th>")
	require.Contains(t, got, "<th>Enabled</th>")
	require.Contains(t, got, "<th>Definition</th>")
	require.Contains(t, got, "<code>trg_audit</code>")
	require.Contains(t, got, "AFTER")
	require.Contains(t, got, "INSERT, UPDATE")

	// View definition renders as <pre><code> with HTML-escaped SQL.
	view := &metadata.Table{
		Name:           "v_enriched",
		TableType:      "view",
		RowCount:       0,
		ViewDefinition: "SELECT id, name FROM enriched WHERE id > 0",
		Columns: []*metadata.Column{
			{Name: "id", Position: 1, ColumnType: "INTEGER"},
		},
	}
	viewBuf := &bytes.Buffer{}
	vw := htmlw.NewMetadataWriter(viewBuf, output.NewPrinting(), false)
	require.NoError(t, vw.TableMetadata(view))
	viewGot := viewBuf.String()
	require.Contains(t, viewGot, "View definition")
	require.Contains(t, viewGot, "<pre><code>")
	require.Contains(t, viewGot, "SELECT id, name FROM enriched WHERE id &gt; 0") // '>' escaped

	// Plain table must NOT emit any of those sections.
	plain := &metadata.Table{
		Name:      "plain",
		TableType: "table",
		RowCount:  1,
		Columns: []*metadata.Column{
			{Name: "id", ColumnType: "INTEGER", PrimaryKey: true},
		},
	}
	plainBuf := &bytes.Buffer{}
	pw := htmlw.NewMetadataWriter(plainBuf, output.NewPrinting(), false)
	require.NoError(t, pw.TableMetadata(plain))
	plainGot := plainBuf.String()
	require.NotContains(t, plainGot, "<th>Auto</th>")
	require.NotContains(t, plainGot, "<th>Generated Expr</th>")
	require.NotContains(t, plainGot, "<th>Collation</th>")
	require.NotContains(t, plainGot, "Check constraints")
	require.NotContains(t, plainGot, "Triggers")
	require.NotContains(t, plainGot, "View definition")
	require.NotContains(t, plainGot, "<pre><code>")
}
