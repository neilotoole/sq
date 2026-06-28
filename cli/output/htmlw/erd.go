package htmlw

import (
	"bytes"
	"cmp"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// SourceMetadata implements output.MetadataWriter.
func (w *metadataWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, md.Handle, showSchema, func(b *bytes.Buffer) {
		w.writeSourceOverview(b, md, showSchema)
		if !showSchema {
			return
		}
		tables := append([]*metadata.Table(nil), md.Tables...)
		slices.SortFunc(tables, compareTables)
		writeMermaidBlock(b, mermaid.SourceDiagram(tables), "erd", 2)
		if len(tables) > 0 {
			byName := mermaid.Index(tables)
			tablesTitle := "Tables"
			if commonw.HasViews(tables) {
				tablesTitle = "Tables & views"
			}
			fmt.Fprintf(b,
				`<h2 id="tables" class="sq-tables"><a class="sq-anchor" href="#tables">%s</a></h2>`+"\n",
				html.EscapeString(tablesTitle))
			writeTablesTOC(b, tables)
			for _, tbl := range tables {
				w.writeTableSection(b, tbl, 3, byName)
			}
		}
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}

// TableMetadata implements output.MetadataWriter.
func (w *metadataWriter) TableMetadata(md *metadata.Table) error {
	buf := &bytes.Buffer{}
	err := w.writeDocument(buf, md.Name, true, func(b *bytes.Buffer) {
		w.writeTableSection(b, md, 1, nil)
	})
	if err != nil {
		return err
	}
	_, err = buf.WriteTo(w.out)
	return err
}

// writeProvenance writes a muted provenance line under the document title,
// "sq inspect @ <timestamp>" (both code; "sq inspect" linked to its docs),
// when the Printing carries a generation timestamp (set by the CLI on every
// real run; absent in unit tests, so output stays deterministic). The
// timestamp is RFC3339 UTC.
func (w *metadataWriter) writeProvenance(buf *bytes.Buffer) {
	if w.pr.GeneratedAt.IsZero() {
		return
	}
	fmt.Fprintf(buf,
		`<p class="sq-gen"><code><a href="https://sq.io/docs/inspect">sq inspect</a></code> `+
			"@ <code>%s</code></p>\n",
		html.EscapeString(w.pr.GeneratedAt.Format(time.RFC3339)))
}

// writeTablesTOC writes a compact, wrapping table-of-contents: a link to each
// table/view section (in the given order), styled as code chips. Each links to
// the section's id (see tableSlug / writeTableHeading).
func writeTablesTOC(buf *bytes.Buffer, tables []*metadata.Table) {
	buf.WriteString("<nav class=\"sq-toc\">\n")
	for _, tbl := range tables {
		class := ""
		if commonw.IsView(tbl) {
			class = ` class="sq-view"`
		}
		fmt.Fprintf(buf, "<a%s href=\"#%s\"><code>%s</code></a>\n",
			class, tableSlug(tbl.Name), html.EscapeString(tbl.Name))
	}
	buf.WriteString("</nav>\n")
}

// writeMermaidBlock writes an "Entity Relationship Diagram" heading (at the
// given level, with a deep-linkable id + hover "#" self-link) and a
// <pre class="mermaid"> block. It writes nothing when src is empty. The diagram
// source is HTML-escaped; the browser decodes it back to text for Mermaid via
// the element's textContent.
func writeMermaidBlock(buf *bytes.Buffer, src, id string, headingLevel int) {
	if src == "" {
		return
	}
	fmt.Fprintf(buf,
		"<h%d id=\"%s\"><a class=\"sq-anchor\" href=\"#%s\">Entity Relationship Diagram</a></h%d>\n",
		headingLevel, id, id, headingLevel)
	buf.WriteString("<pre class=\"mermaid\">\n")
	buf.WriteString(html.EscapeString(src))
	buf.WriteString("</pre>\n")
}

func (w *metadataWriter) writeSourceOverview(buf *bytes.Buffer, md *metadata.Source, showSchema bool) {
	fmt.Fprintf(buf, "<h1>%s</h1>\n", html.EscapeString(md.Handle))
	w.writeProvenance(buf)

	loc := md.Location
	if w.pr.Redact {
		loc = location.Redact(loc)
	}

	rows := [][]string{}
	addRow := func(k, v string) {
		if v != "" {
			rows = append(rows, []string{html.EscapeString(k), htmlCode(v)})
		}
	}
	addRow("Name", md.Name)
	if md.FQName != "" && md.FQName != md.Name {
		addRow("FQ name", md.FQName)
	}
	addRow("Driver", md.Driver.String())
	addRow("DB product", md.DBProduct)
	addRow("DB version", md.DBVersion)
	addRow("Schema", md.Schema)
	addRow("Catalog", md.Catalog)
	addRow("Size", stringz.FormatSize(md.Size))
	if showSchema {
		addRow("Tables", strconv.FormatInt(md.TableCount, 10))
		addRow("Views", strconv.FormatInt(md.ViewCount, 10))
	}
	addRow("Location", loc)
	writeTableEl(buf, "", "", []string{"Property", "Value"}, rows)
}

func (w *metadataWriter) writeTableSection(
	buf *bytes.Buffer, tbl *metadata.Table, level int, cardIndex map[string]*metadata.Table,
) {
	w.writeTableHeading(buf, tbl, level)
	if level == 1 {
		// Single-table document: provenance subtitle under the title (per-table
		// sections within a whole-source doc, at level 3, don't repeat it).
		w.writeProvenance(buf)
	}
	writeMermaidBlock(buf, mermaid.TableDiagram(tbl, cardIndex), tableSlug(tbl.Name)+"-erd", level+1)
	w.writeViewDefinition(buf, tbl)
	w.writeColumns(buf, tbl)
	w.writeForeignKeys(buf, tbl)
	w.writeUniqueConstraints(buf, tbl)
	w.writeIndexes(buf, tbl)
	w.writeCheckConstraints(buf, tbl)
	w.writeTriggers(buf, tbl)
}

func (w *metadataWriter) writeTableHeading(buf *bytes.Buffer, tbl *metadata.Table, level int) {
	// The heading carries an id (e.g. #city) so a table's section is
	// deep-linkable, and is a self-link that reveals a "#" on hover. The
	// sq-table class adds the section-separator + larger treatment, but only
	// for per-table sections within a larger doc (level > 1); a single-table
	// doc's level-1 heading is already the document title.
	slug := tableSlug(tbl.Name)
	class := ""
	if level > 1 {
		class = ` class="sq-table"`
	}
	fmt.Fprintf(buf,
		"<h%d id=\"%s\"%s><a class=\"sq-anchor\" href=\"#%s\"><code>%s</code></a></h%d>\n",
		level, slug, class, slug, html.EscapeString(tbl.Name), level)

	typ := tbl.TableType
	if typ == "" {
		typ = "table"
	}
	summary := fmt.Sprintf("%s · %s rows", typ, strconv.FormatInt(tbl.RowCount, 10))
	if tbl.Size != nil {
		summary += " · " + stringz.ByteSized(*tbl.Size, 1, "")
	}
	fmt.Fprintf(buf, "<p class=\"summary\">%s</p>\n", html.EscapeString(summary))
	if tbl.Comment != "" {
		fmt.Fprintf(buf, "<p class=\"comment\"><em>%s</em></p>\n", html.EscapeString(tbl.Comment))
	}
}

func (w *metadataWriter) writeColumns(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Columns) == 0 {
		return
	}
	fkCols := commonw.FKColumnSet(tbl)

	// Only include optional columns when at least one column populates them,
	// keeping the common case clean.
	var hasDefault, hasComment bool
	var hasAuto, hasGeneratedExpr, hasCollation bool
	for _, col := range tbl.Columns {
		hasDefault = hasDefault || col.DefaultValue != ""
		hasComment = hasComment || col.Comment != ""
		hasAuto = hasAuto || col.Identity || col.AutoIncrement || col.Generated
		hasGeneratedExpr = hasGeneratedExpr || col.GeneratedExpr != ""
		hasCollation = hasCollation || col.Collation != ""
	}
	headers := []string{"Column", "Type", "Nullable", "PK", "FK"}
	if hasAuto {
		headers = append(headers, "Auto")
	}
	if hasGeneratedExpr {
		headers = append(headers, "Generated Expr")
	}
	if hasCollation {
		headers = append(headers, "Collation")
	}
	if hasDefault {
		headers = append(headers, "Default")
	}
	if hasComment {
		headers = append(headers, "Comment")
	}

	var rows [][]string
	for _, col := range tbl.Columns {
		row := []string{
			htmlCode(col.Name),
			htmlCode(col.ColumnType),
			checkMark(col.Nullable),
			checkMark(col.PrimaryKey),
			checkMark(fkCols[col.Name]),
		}
		if hasAuto {
			row = append(row, commonw.ColumnAutoLabel(col))
		}
		if hasGeneratedExpr {
			row = append(row, htmlCode(col.GeneratedExpr))
		}
		if hasCollation {
			row = append(row, html.EscapeString(col.Collation))
		}
		if hasDefault {
			row = append(row, htmlCode(col.DefaultValue))
		}
		if hasComment {
			row = append(row, html.EscapeString(col.Comment))
		}
		rows = append(rows, row)
	}
	writeTableEl(buf, "Columns", tableSlug(tbl.Name)+"-columns", headers, rows)
}

func (w *metadataWriter) writeForeignKeys(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.FKRows(tbl)
	if len(rows) == 0 {
		return
	}

	relHeader := `Relationship <span class="fk-legend">(` +
		`<span class="fk-out">→</span> references · ` +
		`<span class="fk-in">←</span> referenced by)</span>`
	headers := []string{relHeader, "Constraint", "On update", "On delete"}
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		arrow := `<span class="fk-out">→</span>`
		if r.Direction == "incoming" {
			arrow = `<span class="fk-in">←</span>`
		}
		rel := htmlCode(r.Local) + " " + arrow + " " + htmlCode(r.Remote)
		cells = append(cells, []string{
			rel,
			htmlCode(r.Constraint),
			html.EscapeString(r.OnUpdate),
			html.EscapeString(r.OnDelete),
		})
	}
	writeTableEl(buf, "Foreign keys", tableSlug(tbl.Name)+"-foreign-keys", headers, cells)
}

func (w *metadataWriter) writeUniqueConstraints(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.UCRows(tbl)
	if len(rows) == 0 {
		return
	}
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, []string{htmlCode(r.Name), htmlCode(r.Columns)})
	}
	writeTableEl(buf, "Unique constraints", tableSlug(tbl.Name)+"-unique-constraints",
		[]string{"Constraint", "Columns"}, cells)
}

func (w *metadataWriter) writeIndexes(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.IndexRows(tbl)
	if len(rows) == 0 {
		return
	}
	headers := []string{"Index", "Columns", "Unique", "Primary", "Type"}
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, []string{
			htmlCode(r.Name),
			htmlCode(r.Columns),
			checkMark(r.Unique),
			checkMark(r.Primary),
			html.EscapeString(r.Type),
		})
	}
	writeTableEl(buf, "Indexes", tableSlug(tbl.Name)+"-indexes", headers, cells)
}

// writeViewDefinition renders the raw view DDL as a <pre><code> block with
// a "View definition:" heading when tbl has a non-empty ViewDefinition. All
// content is HTML-escaped.
func (w *metadataWriter) writeViewDefinition(buf *bytes.Buffer, tbl *metadata.Table) {
	if tbl.ViewDefinition == "" {
		return
	}
	buf.WriteString("<p><strong>View definition:</strong></p>\n")
	buf.WriteString("<pre><code>")
	buf.WriteString(html.EscapeString(tbl.ViewDefinition))
	buf.WriteString("</code></pre>\n")
}

// writeCheckConstraints renders a "Check constraints" table (Constraint/Clause
// columns) when tbl.CheckConstraints is non-empty.
func (w *metadataWriter) writeCheckConstraints(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.CheckConstraints) == 0 {
		return
	}
	cells := make([][]string, 0, len(tbl.CheckConstraints))
	for _, cc := range tbl.CheckConstraints {
		cells = append(cells, []string{htmlCode(cc.Name), html.EscapeString(cc.Clause)})
	}
	writeTableEl(buf, "Check constraints", tableSlug(tbl.Name)+"-check-constraints",
		[]string{"Constraint", "Clause"}, cells)
}

// writeTriggers renders a "Triggers" table (Trigger/Timing/Events, with
// optional Enabled and Definition columns) when tbl.Triggers is non-empty.
// Enabled is included only when at least one trigger carries a non-nil value;
// Definition only when at least one trigger has a non-empty Definition.
func (w *metadataWriter) writeTriggers(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Triggers) == 0 {
		return
	}
	var hasEnabled, hasDefinition bool
	for _, tr := range tbl.Triggers {
		hasEnabled = hasEnabled || tr.Enabled != nil
		hasDefinition = hasDefinition || tr.Definition != ""
	}
	headers := []string{"Trigger", "Timing", "Events"}
	if hasEnabled {
		headers = append(headers, "Enabled")
	}
	if hasDefinition {
		headers = append(headers, "Definition")
	}
	cells := make([][]string, 0, len(tbl.Triggers))
	for _, tr := range tbl.Triggers {
		row := []string{
			htmlCode(tr.Name),
			html.EscapeString(tr.Timing),
			html.EscapeString(strings.Join(tr.Events, ", ")),
		}
		if hasEnabled {
			row = append(row, commonw.TriggerEnabledMark(tr.Enabled))
		}
		if hasDefinition {
			row = append(row, htmlCode(tr.Definition))
		}
		cells = append(cells, row)
	}
	writeTableEl(buf, "Triggers", tableSlug(tbl.Name)+"-triggers", headers, cells)
}

func compareTables(a, b *metadata.Table) int {
	if a.TableType == b.TableType {
		return cmp.Compare(a.Name, b.Name)
	}
	return cmp.Compare(a.TableType, b.TableType)
}

// tableSlug returns an HTML id / URL fragment for a table name: characters
// outside [A-Za-z0-9_-] are replaced with '-' (case preserved), so common
// names like "city" and "film_actor" are used verbatim (#city, #film_actor)
// while unusual names stay valid and space-free. Table names are unique within
// a schema, so the resulting ids are unique within a document.
func tableSlug(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r == '_' || r == '-',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}
