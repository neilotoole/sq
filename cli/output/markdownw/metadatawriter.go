package markdownw

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/output"
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
			byName := tableIndex(tables)
			buf.WriteString("\n## Tables\n")
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
	w.writeTableERD(buf, md, 2, nil)
	w.writeTableBody(buf, md)

	_, err := buf.WriteTo(w.out)
	return err
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
		writeTableRow(buf,
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
	writeKVRow(buf, "Size", stringz.ByteSized(md.Size, 1, ""))
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

// writeTableBody writes the per-table detail: a column table plus
// foreign-key, unique-constraint, and index sections (each omitted when
// empty).
func (w *metadataWriter) writeTableBody(buf *bytes.Buffer, tbl *metadata.Table) {
	w.writeColumns(buf, tbl)
	w.writeForeignKeys(buf, tbl)
	w.writeUniqueConstraints(buf, tbl)
	w.writeIndexes(buf, tbl)
}

func (w *metadataWriter) writeColumns(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Columns) == 0 {
		return
	}

	fkCols := fkColumnSet(tbl)
	ucCols := ucColumnSet(tbl)

	// Only include the Default / Comment columns when at least one
	// column populates them, keeping the common case clean.
	var hasDefault, hasComment bool
	for _, col := range tbl.Columns {
		hasDefault = hasDefault || col.DefaultValue != ""
		hasComment = hasComment || col.Comment != ""
	}

	headers := []string{"Column", "Type", "Null", "Key"}
	aligns := []string{"---", "---", ":---:", ":---:"}
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
			yesNo(col.Nullable),
			mdCodeCell(columnKey(col, fkCols, ucCols)),
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
	if tbl.FK != nil && len(tbl.FK.Outgoing) > 0 {
		fks := append([]*metadata.ForeignKey(nil), tbl.FK.Outgoing...)
		slices.SortFunc(fks, compareFK)

		buf.WriteString("\n**Foreign keys:**\n\n")
		for _, fk := range fks {
			line := fmt.Sprintf("- `%s` → `%s`", strings.Join(fk.Columns, ", "), formatFKRef(fk))
			if extra := fkExtra(fk); extra != "" {
				line += " (" + extra + ")"
			}
			buf.WriteString(line + "\n")
		}
	}

	if tbl.FK != nil && len(tbl.FK.Incoming) > 0 {
		fks := append([]*metadata.ForeignKey(nil), tbl.FK.Incoming...)
		slices.SortFunc(fks, compareFK)

		buf.WriteString("\n**Referenced by:**\n\n")
		for _, fk := range fks {
			fmt.Fprintf(buf, "- `%s(%s)` → `%s`\n",
				fk.Table, strings.Join(fk.Columns, ", "), formatFKRef(fk))
		}
	}
}

func (w *metadataWriter) writeUniqueConstraints(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.UniqueConstraints) == 0 {
		return
	}

	ucs := append([]*metadata.UniqueConstraint(nil), tbl.UniqueConstraints...)
	slices.SortFunc(ucs, func(a, b *metadata.UniqueConstraint) int {
		if c := cmp.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return cmp.Compare(strings.Join(a.Columns, ","), strings.Join(b.Columns, ","))
	})

	buf.WriteString("\n**Unique constraints:**\n\n")
	for _, uc := range ucs {
		name := uc.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Fprintf(buf, "- `%s` (%s)\n", name, strings.Join(uc.Columns, ", "))
	}
}

func (w *metadataWriter) writeIndexes(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Indexes) == 0 {
		return
	}

	idxs := append([]*metadata.Index(nil), tbl.Indexes...)
	slices.SortFunc(idxs, func(a, b *metadata.Index) int {
		return cmp.Compare(a.Name, b.Name)
	})

	buf.WriteString("\n**Indexes:**\n\n")
	for _, idx := range idxs {
		var tags []string
		switch {
		case idx.Primary:
			tags = append(tags, "primary")
		case idx.Unique:
			tags = append(tags, "unique")
		}
		if idx.Type != "" {
			tags = append(tags, strings.ToLower(idx.Type))
		}

		line := fmt.Sprintf("- `%s` (%s)", idx.Name, strings.Join(idx.Columns, ", "))
		if len(tags) > 0 {
			line += " — " + strings.Join(tags, ", ")
		}
		buf.WriteString(line + "\n")
	}
}

// fkExtra returns a parenthetical summary of a foreign key's constraint
// name and referential actions, or "" when none are reported.
func fkExtra(fk *metadata.ForeignKey) string {
	var parts []string
	if fk.Name != "" {
		parts = append(parts, "constraint `"+fk.Name+"`")
	}
	if fk.OnDelete != "" {
		parts = append(parts, "on delete "+strings.ToLower(fk.OnDelete))
	}
	if fk.OnUpdate != "" {
		parts = append(parts, "on update "+strings.ToLower(fk.OnUpdate))
	}
	return strings.Join(parts, ", ")
}

// formatFKRef returns "ref_table(ref_col, ...)" for fk, qualified with
// the referenced schema/catalog when the reference points outside this
// source. Same-source references stay unqualified because
// [metadata.LinkForeignKeys] clears RefCatalog / RefSchema when they
// match the owning source.
func formatFKRef(fk *metadata.ForeignKey) string {
	if fk == nil {
		return ""
	}
	target := fk.RefTable
	if fk.RefSchema != "" {
		target = fk.RefSchema + "." + target
	}
	if fk.RefCatalog != "" {
		target = fk.RefCatalog + "." + target
	}
	return target + "(" + strings.Join(fk.RefColumns, ", ") + ")"
}

// columnKey returns the combined "PK,FK,UK" marker for a column, or ""
// when the column participates in no key.
func columnKey(col *metadata.Column, fkCols, ucCols map[string]bool) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if fkCols[col.Name] {
		parts = append(parts, "FK")
	}
	if ucCols[col.Name] {
		parts = append(parts, "UK")
	}
	return strings.Join(parts, ",")
}

// fkColumnSet returns the set of column names on tbl that participate in
// any outgoing foreign key.
func fkColumnSet(tbl *metadata.Table) map[string]bool {
	if tbl.FK == nil {
		return nil
	}
	set := make(map[string]bool)
	for _, fk := range tbl.FK.Outgoing {
		if fk == nil {
			continue
		}
		for _, c := range fk.Columns {
			set[c] = true
		}
	}
	return set
}

// ucColumnSet returns the set of column names on tbl that participate in
// any unique constraint.
func ucColumnSet(tbl *metadata.Table) map[string]bool {
	if len(tbl.UniqueConstraints) == 0 {
		return nil
	}
	set := make(map[string]bool)
	for _, uc := range tbl.UniqueConstraints {
		if uc == nil {
			continue
		}
		for _, c := range uc.Columns {
			set[c] = true
		}
	}
	return set
}

func compareTables(a, b *metadata.Table) int {
	if a.TableType == b.TableType {
		return cmp.Compare(a.Name, b.Name)
	}
	return cmp.Compare(a.TableType, b.TableType)
}

func compareFK(a, b *metadata.ForeignKey) int {
	if c := cmp.Compare(a.Name, b.Name); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Table, b.Table); c != 0 {
		return c
	}
	if c := cmp.Compare(a.RefTable, b.RefTable); c != 0 {
		return c
	}
	return cmp.Compare(strings.Join(a.Columns, ","), strings.Join(b.Columns, ","))
}

// writeKVRow writes a "| key | value |" row, skipping empty values.
func writeKVRow(buf *bytes.Buffer, k, v string) {
	if v == "" {
		return
	}
	writeTableRow(buf, k, escapeMarkdown(v))
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
