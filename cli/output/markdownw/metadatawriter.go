package markdownw

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ output.MetadataWriter = (*metadataWriter)(nil)

// metadataWriter implements output.MetadataWriter for Markdown. It
// renders source and table metadata as a Markdown document, including
// a Mermaid.js entity-relationship diagram. The type is named
// metadataWriter (not the "mdWriter" used by the other format packages)
// to avoid conflating "md" = metadata with "md" = Markdown.
type metadataWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewMetadataWriter returns a new output.MetadataWriter instance that
// outputs metadata as a Markdown document.
func NewMetadataWriter(out io.Writer, pr *output.Printing) output.MetadataWriter {
	return &metadataWriter{out: out, pr: pr}
}

// SourceMetadata implements output.MetadataWriter.
func (w *metadataWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	buf := &bytes.Buffer{}
	w.writeSourceOverview(buf, md, showSchema)

	if showSchema {
		// Render with a stable table ordering (tables before views, then
		// by name) so both the ERD and the per-table sections — and the
		// test golden output — are deterministic.
		tables := append([]*metadata.Table(nil), md.Tables...)
		slices.SortFunc(tables, compareTables)

		w.writeSourceERD(buf, tables)

		if len(tables) > 0 {
			// Every table is known here, so its per-table ERD can infer
			// cardinality consistently with the whole-source diagram above.
			byName := mermaid.Index(tables)
			tablesTitle := "Tables"
			if commonw.HasViews(tables) {
				tablesTitle = "Tables & views"
			}
			fmt.Fprintf(buf, "\n## %s\n\n", tablesTitle)
			writeTablesTOC(buf, tables)
			for _, tbl := range tables {
				buf.WriteString("\n")
				writeTableHeading(buf, tbl, 3)
				w.writeTableERD(buf, tbl, 4, byName)
				w.writeTableBody(buf, tbl)
			}
		}
	}

	_, err := buf.WriteTo(w.out)
	return err
}

// TableMetadata implements output.MetadataWriter.
func (w *metadataWriter) TableMetadata(md *metadata.Table) error {
	buf := &bytes.Buffer{}
	writeTableHeading(buf, md, 1)
	w.writeProvenance(buf)
	w.writeTableERD(buf, md, 2, nil)
	w.writeTableBody(buf, md)

	_, err := buf.WriteTo(w.out)
	return err
}

// writeProvenance writes an italic provenance line under the document title,
// "`sq inspect` @ `<timestamp>`" (the command linked to its docs), when the
// Printing carries a generation timestamp (set by the CLI on every real run;
// absent in unit tests, so goldens stay deterministic). The timestamp is
// RFC3339 UTC.
func (w *metadataWriter) writeProvenance(buf *bytes.Buffer) {
	if w.pr.GeneratedAt.IsZero() {
		return
	}
	fmt.Fprintf(buf, "*[`sq inspect`](https://sq.io/docs/inspect) @ `%s`*\n\n",
		w.pr.GeneratedAt.Format(time.RFC3339))
}

// DBProperties implements output.MetadataWriter.
func (w *metadataWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	buf.WriteString("| Property | Value |\n| --- | --- |\n")

	keys := lo.Keys(props)
	slices.Sort(keys)
	for _, k := range keys {
		v := props[k]
		if v == nil {
			continue
		}
		// Most properties are scalars; nested values (maps/slices) are
		// rendered compactly via %v. YAML/JSON output is the better
		// choice for deeply nested DB properties.
		writeTableRow(buf, escapeMarkdown(k), escapeMarkdown(fmt.Sprintf("%v", v)))
	}

	_, err := buf.WriteTo(w.out)
	return err
}

// DriverMetadata implements output.MetadataWriter.
func (w *metadataWriter) DriverMetadata(drvrs []driver.Metadata) error {
	if len(drvrs) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	buf.WriteString("| Driver | Description | User-defined |\n| --- | --- | :---: |\n")
	for _, md := range drvrs {
		writeTableRow(
			buf,
			escapeMarkdown(string(md.Type)),
			escapeMarkdown(md.Description),
			yesNo(md.UserDefined),
		)
	}

	_, err := buf.WriteTo(w.out)
	return err
}

// Catalogs implements output.MetadataWriter.
func (w *metadataWriter) Catalogs(currentCatalog string, catalogs []string) error {
	if len(catalogs) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	buf.WriteString("| Catalog | Active |\n| --- | :---: |\n")
	for _, c := range catalogs {
		var active string
		if c == currentCatalog {
			active = "✓"
		}
		writeTableRow(buf, escapeMarkdown(c), active)
	}

	_, err := buf.WriteTo(w.out)
	return err
}

// Schemata implements output.MetadataWriter.
func (w *metadataWriter) Schemata(currentSchema string, schemas []*metadata.Schema) error {
	if len(schemas) == 0 {
		return nil
	}

	buf := &bytes.Buffer{}
	buf.WriteString("| Schema | Catalog | Owner | Active |\n| --- | --- | --- | :---: |\n")
	for _, s := range schemas {
		var active string
		if s.Name == currentSchema {
			active = "✓"
		}
		writeTableRow(buf, escapeMarkdown(s.Name), escapeMarkdown(s.Catalog), escapeMarkdown(s.Owner), active)
	}

	_, err := buf.WriteTo(w.out)
	return err
}

// writeSourceOverview writes the source's "# @handle" heading followed
// by a property table. Table/view counts are only emitted when
// showSchema is true; in overview mode they aren't populated (matching
// the text and YAML writers, which also omit them there).
func (w *metadataWriter) writeSourceOverview(buf *bytes.Buffer, md *metadata.Source, showSchema bool) {
	fmt.Fprintf(buf, "# %s\n\n", md.Handle)
	w.writeProvenance(buf)

	loc := md.Location
	if w.pr.Redact {
		loc = location.Redact(loc)
	}

	buf.WriteString("| Property | Value |\n| --- | --- |\n")
	writeKVRow(buf, "Name", md.Name)
	if md.FQName != "" && md.FQName != md.Name {
		writeKVRow(buf, "FQ name", md.FQName)
	}
	writeKVRow(buf, "Driver", md.Driver.String())
	writeKVRow(buf, "DB product", md.DBProduct)
	writeKVRow(buf, "DB version", md.DBVersion)
	writeKVRow(buf, "Schema", md.Schema)
	writeKVRow(buf, "Catalog", md.Catalog)
	writeKVRow(buf, "Size", stringz.FormatSize(md.Size))
	if showSchema {
		writeKVRow(buf, "Tables", strconv.FormatInt(md.TableCount, 10))
		writeKVRow(buf, "Views", strconv.FormatInt(md.ViewCount, 10))
	}
	writeKVRow(buf, "Location", loc)
}

// writeTableHeading writes a heading (at the given '#' level) for tbl,
// followed by a one-line summary and the table comment (if any).
func writeTableHeading(buf *bytes.Buffer, tbl *metadata.Table, level int) {
	fmt.Fprintf(buf, "%s %s\n\n", strings.Repeat("#", level), mdCode(tbl.Name))

	typ := tbl.TableType
	if typ == "" {
		typ = "table"
	}
	summary := fmt.Sprintf("%s · %s rows", typ, strconv.FormatInt(tbl.RowCount, 10))
	if tbl.Size != nil {
		summary += " · " + stringz.ByteSized(*tbl.Size, 1, "")
	}
	buf.WriteString(summary + "\n")

	if tbl.Comment != "" {
		fmt.Fprintf(buf, "\n_%s_\n", escapeMarkdown(tbl.Comment))
	}
}

// writeTableBody writes the per-table detail: an optional view-definition
// block, a column table, and foreign-key, unique-constraint, index,
// check-constraint, and trigger sections (each omitted when empty).
func (w *metadataWriter) writeTableBody(buf *bytes.Buffer, tbl *metadata.Table) {
	w.writeViewDefinition(buf, tbl)
	w.writeColumns(buf, tbl)
	w.writeForeignKeys(buf, tbl)
	w.writeUniqueConstraints(buf, tbl)
	w.writeIndexes(buf, tbl)
	w.writeCheckConstraints(buf, tbl)
	w.writeTriggers(buf, tbl)
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
	aligns := []string{"---", "---", ":---:", ":---:", ":---:"}
	if hasAuto {
		headers = append(headers, "Auto")
		aligns = append(aligns, "---")
	}
	if hasGeneratedExpr {
		headers = append(headers, "Generated Expr")
		aligns = append(aligns, "---")
	}
	if hasCollation {
		headers = append(headers, "Collation")
		aligns = append(aligns, "---")
	}
	if hasDefault {
		headers = append(headers, "Default")
		aligns = append(aligns, "---")
	}
	if hasComment {
		headers = append(headers, "Comment")
		aligns = append(aligns, "---")
	}

	buf.WriteString("\n")
	writeTableRow(buf, headers...)
	writeTableRow(buf, aligns...)

	for _, col := range tbl.Columns {
		cells := []string{
			mdCodeCell(col.Name),
			mdCodeCell(col.ColumnType),
			checkMark(col.Nullable),
			checkMark(col.PrimaryKey),
			checkMark(fkCols[col.Name]),
		}
		if hasAuto {
			cells = append(cells, commonw.ColumnAutoLabel(col))
		}
		if hasGeneratedExpr {
			cells = append(cells, mdCodeCell(col.GeneratedExpr))
		}
		if hasCollation {
			cells = append(cells, escapeMarkdown(col.Collation))
		}
		if hasDefault {
			cells = append(cells, mdCodeCell(col.DefaultValue))
		}
		if hasComment {
			cells = append(cells, escapeMarkdown(col.Comment))
		}
		writeTableRow(buf, cells...)
	}
}

func (w *metadataWriter) writeForeignKeys(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.FKRows(tbl)
	if len(rows) == 0 {
		return
	}

	buf.WriteString("\n**Foreign keys:**\n\n")
	writeTableRow(buf, "Relationship (→ references · ← referenced by)", "Constraint", "On update", "On delete")
	writeTableRow(buf, "---", "---", "---", "---")
	for _, r := range rows {
		arrow := "→"
		if r.Direction == "incoming" {
			arrow = "←"
		}
		rel := mdCodeCell(r.Local) + " " + arrow + " " + mdCodeCell(r.Remote)
		writeTableRow(buf, rel, mdCodeCell(r.Constraint), r.OnUpdate, r.OnDelete)
	}
}

func (w *metadataWriter) writeUniqueConstraints(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.UCRows(tbl)
	if len(rows) == 0 {
		return
	}

	buf.WriteString("\n**Unique constraints:**\n\n")
	writeTableRow(buf, "Constraint", "Columns")
	writeTableRow(buf, "---", "---")
	for _, r := range rows {
		writeTableRow(buf, mdCodeCell(r.Name), mdCodeCell(r.Columns))
	}
}

func (w *metadataWriter) writeIndexes(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.IndexRows(tbl)
	if len(rows) == 0 {
		return
	}

	buf.WriteString("\n**Indexes:**\n\n")
	writeTableRow(buf, "Index", "Columns", "Unique", "Primary", "Type")
	writeTableRow(buf, "---", "---", ":---:", ":---:", "---")
	for _, r := range rows {
		writeTableRow(
			buf,
			mdCodeCell(r.Name),
			mdCodeCell(r.Columns),
			checkMark(r.Unique),
			checkMark(r.Primary),
			r.Type,
		)
	}
}

// writeViewDefinition renders the raw view DDL as a fenced SQL code block
// when tbl is a view/materialized_view with a non-empty ViewDefinition.
func (w *metadataWriter) writeViewDefinition(buf *bytes.Buffer, tbl *metadata.Table) {
	if tbl.ViewDefinition == "" {
		return
	}
	buf.WriteString("\n**View definition:**\n\n```sql\n")
	buf.WriteString(tbl.ViewDefinition)
	if !strings.HasSuffix(tbl.ViewDefinition, "\n") {
		buf.WriteByte('\n')
	}
	buf.WriteString("```\n")
}

// writeCheckConstraints renders a "Check constraints" subsection (table of
// name + clause) when tbl.CheckConstraints is non-empty.
func (w *metadataWriter) writeCheckConstraints(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.CheckConstraints) == 0 {
		return
	}

	buf.WriteString("\n**Check constraints:**\n\n")
	writeTableRow(buf, "Constraint", "Clause")
	writeTableRow(buf, "---", "---")
	for _, cc := range tbl.CheckConstraints {
		writeTableRow(buf, mdCodeCell(cc.Name), escapeMarkdown(cc.Clause))
	}
}

// writeTriggers renders a "Triggers" subsection (name, timing, events,
// enabled, definition) when tbl.Triggers is non-empty. The "Enabled" column
// is only emitted when at least one trigger carries a non-nil Enabled value;
// the "Definition" column is only emitted when at least one trigger has a
// non-empty Definition.
func (w *metadataWriter) writeTriggers(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Triggers) == 0 {
		return
	}

	var hasEnabled, hasDefinition bool
	for _, tr := range tbl.Triggers {
		hasEnabled = hasEnabled || tr.Enabled != nil
		hasDefinition = hasDefinition || tr.Definition != ""
	}

	buf.WriteString("\n**Triggers:**\n\n")
	headers := []string{"Trigger", "Timing", "Events"}
	aligns := []string{"---", "---", "---"}
	if hasEnabled {
		headers = append(headers, "Enabled")
		aligns = append(aligns, ":---:")
	}
	if hasDefinition {
		headers = append(headers, "Definition")
		aligns = append(aligns, "---")
	}
	writeTableRow(buf, headers...)
	writeTableRow(buf, aligns...)

	for _, tr := range tbl.Triggers {
		cells := []string{
			mdCodeCell(tr.Name),
			escapeMarkdown(tr.Timing),
			escapeMarkdown(strings.Join(tr.Events, ", ")),
		}
		if hasEnabled {
			cells = append(cells, commonw.TriggerEnabledMark(tr.Enabled))
		}
		if hasDefinition {
			cells = append(cells, mdCodeCell(tr.Definition))
		}
		writeTableRow(buf, cells...)
	}
}

func compareTables(a, b *metadata.Table) int {
	if a.TableType == b.TableType {
		return cmp.Compare(a.Name, b.Name)
	}
	return cmp.Compare(a.TableType, b.TableType)
}

// writeKVRow writes a "| key | value |" row, skipping empty values. The value
// is rendered as inline code (the key is a plain label).
func writeKVRow(buf *bytes.Buffer, k, v string) {
	if v == "" {
		return
	}
	writeTableRow(buf, k, mdCodeCell(v))
}

// writeTableRow writes a single Markdown table row from cells.
func writeTableRow(buf *bytes.Buffer, cells ...string) {
	buf.WriteString("| ")
	buf.WriteString(strings.Join(cells, " | "))
	buf.WriteString(" |\n")
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
