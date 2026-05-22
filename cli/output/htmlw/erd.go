package htmlw

import (
	"bytes"
	"cmp"
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

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
		writeMermaidBlock(b, mermaid.SourceDiagram(tables), 2)
		if len(tables) > 0 {
			byName := mermaid.Index(tables)
			b.WriteString("<h2>Tables</h2>\n")
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

// writeMermaidBlock writes an "Entity Relationship Diagram" heading (at the
// given level) and a <pre class="mermaid"> block. It writes nothing when src
// is empty. The diagram source is HTML-escaped; the browser decodes it back
// to text for Mermaid via the element's textContent.
func writeMermaidBlock(buf *bytes.Buffer, src string, headingLevel int) {
	if src == "" {
		return
	}
	fmt.Fprintf(buf, "<h%d>Entity Relationship Diagram</h%d>\n", headingLevel, headingLevel)
	buf.WriteString("<pre class=\"mermaid\">\n")
	buf.WriteString(html.EscapeString(src))
	buf.WriteString("</pre>\n")
}

func (w *metadataWriter) writeSourceOverview(buf *bytes.Buffer, md *metadata.Source, showSchema bool) {
	fmt.Fprintf(buf, "<h1>%s</h1>\n", html.EscapeString(md.Handle))

	loc := md.Location
	if w.pr.Redact {
		loc = location.Redact(loc)
	}

	rows := [][]string{}
	addRow := func(k, v string) {
		if v != "" {
			rows = append(rows, []string{html.EscapeString(k), html.EscapeString(v)})
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
	addRow("Size", stringz.ByteSized(md.Size, 1, ""))
	if showSchema {
		addRow("Tables", strconv.FormatInt(md.TableCount, 10))
		addRow("Views", strconv.FormatInt(md.ViewCount, 10))
	}
	addRow("Location", loc)
	writeTableEl(buf, []string{"Property", "Value"}, rows)
}

func (w *metadataWriter) writeTableSection(
	buf *bytes.Buffer, tbl *metadata.Table, level int, cardIndex map[string]*metadata.Table,
) {
	w.writeTableHeading(buf, tbl, level)
	writeMermaidBlock(buf, mermaid.TableDiagram(tbl, cardIndex), level+1)
	w.writeColumns(buf, tbl)
	w.writeForeignKeys(buf, tbl)
	w.writeUniqueConstraints(buf, tbl)
	w.writeIndexes(buf, tbl)
}

func (w *metadataWriter) writeTableHeading(buf *bytes.Buffer, tbl *metadata.Table, level int) {
	fmt.Fprintf(buf, "<h%d><code>%s</code></h%d>\n", level, html.EscapeString(tbl.Name), level)

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
	ucCols := commonw.UCColumnSet(tbl)

	var hasDefault, hasComment bool
	for _, col := range tbl.Columns {
		hasDefault = hasDefault || col.DefaultValue != ""
		hasComment = hasComment || col.Comment != ""
	}
	headers := []string{"Column", "Type", "Null", "Key"}
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
			yesNo(col.Nullable),
			htmlCode(commonw.ColumnKey(col, fkCols, ucCols)),
		}
		if hasDefault {
			row = append(row, htmlCode(col.DefaultValue))
		}
		if hasComment {
			row = append(row, html.EscapeString(col.Comment))
		}
		rows = append(rows, row)
	}
	writeTableEl(buf, headers, rows)
}

func (w *metadataWriter) writeForeignKeys(buf *bytes.Buffer, tbl *metadata.Table) {
	rows := commonw.FKRows(tbl)
	if len(rows) == 0 {
		return
	}

	buf.WriteString("<p><strong>Foreign keys:</strong></p>\n")
	headers := []string{"Direction", "From", "To", "Constraint", "On update", "On delete"}
	cells := make([][]string, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, []string{
			html.EscapeString(r.Direction),
			htmlCode(r.From),
			htmlCode(r.To),
			htmlCode(r.Constraint),
			html.EscapeString(r.OnUpdate),
			html.EscapeString(r.OnDelete),
		})
	}
	writeTableEl(buf, headers, cells)
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
	buf.WriteString("<p><strong>Unique constraints:</strong></p>\n<ul>\n")
	for _, uc := range ucs {
		name := uc.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Fprintf(buf, "<li>%s (%s)</li>\n",
			htmlCode(name), html.EscapeString(strings.Join(uc.Columns, ", ")))
	}
	buf.WriteString("</ul>\n")
}

func (w *metadataWriter) writeIndexes(buf *bytes.Buffer, tbl *metadata.Table) {
	if len(tbl.Indexes) == 0 {
		return
	}
	idxs := append([]*metadata.Index(nil), tbl.Indexes...)
	slices.SortFunc(idxs, func(a, b *metadata.Index) int { return cmp.Compare(a.Name, b.Name) })
	buf.WriteString("<p><strong>Indexes:</strong></p>\n<ul>\n")
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
		line := fmt.Sprintf("<li>%s (%s)",
			htmlCode(idx.Name), html.EscapeString(strings.Join(idx.Columns, ", ")))
		if len(tags) > 0 {
			line += " — " + html.EscapeString(strings.Join(tags, ", "))
		}
		buf.WriteString(line + "</li>\n")
	}
	buf.WriteString("</ul>\n")
}

func compareTables(a, b *metadata.Table) int {
	if a.TableType == b.TableType {
		return cmp.Compare(a.Name, b.Name)
	}
	return cmp.Compare(a.TableType, b.TableType)
}
