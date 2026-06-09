package explore

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// detailKind selects which sub-view detailPane renders.
type detailKind int

const (
	detailNone detailKind = iota
	detailSource
	detailTable
	detailColumn
)

// detailPane is the rightmost pane.
type detailPane struct {
	src     *metadata.Source
	tbl     *metadata.Table
	col     *metadata.Column
	preview *previewBuffer
	theme   theme
	kind    detailKind
}

func newDetailPane(th theme) *detailPane { return &detailPane{theme: th} }

func (d *detailPane) setSource(s *metadata.Source) {
	d.kind = detailSource
	d.src = s
	d.tbl = nil
	d.col = nil
}

func (d *detailPane) setTable(t *metadata.Table) {
	d.kind = detailTable
	d.tbl = t
	d.col = nil
}

// setColumn switches the pane to column-detail mode.
func (d *detailPane) setColumn(c *metadata.Column) {
	d.kind = detailColumn
	d.col = c
}

// setPreview stores the preview buffer to render under the detail pane.
// Replaces any earlier preview.
func (d *detailPane) setPreview(p *previewBuffer) { d.preview = p }

// view renders the appropriate sub-view at width/height. The focused
// flag selects the focused border style.
func (d *detailPane) view(focused bool, width, height int) string {
	var body string
	switch d.kind {
	case detailNone:
		body = d.theme.Faint.Render("(loading)")
	case detailSource:
		body = d.viewSource()
	case detailTable:
		body = d.viewTable()
	case detailColumn:
		body = d.viewColumn()
	default:
		body = d.theme.Faint.Render("(loading)")
	}
	style := d.theme.Pane
	if focused {
		style = d.theme.PaneFocus
	}
	return style.Width(width).Height(height).MaxHeight(height).Render(body)
}

func (d *detailPane) viewSource() string {
	if d.src == nil {
		return d.theme.Faint.Render("(loading)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.theme.Handle.Render(d.src.Handle))
	switch {
	case d.src.DBProduct != "":
		fmt.Fprintf(&b, "%s %s\n", d.theme.Faint.Render("driver:"), d.src.DBProduct)
	case d.src.Driver != "":
		fmt.Fprintf(&b, "%s %s\n", d.theme.Faint.Render("driver:"), d.src.Driver)
	}
	if d.src.Location != "" {
		fmt.Fprintf(&b, "%s %s\n", d.theme.Faint.Render("location:"), d.theme.Location.Render(d.src.Location))
	}
	d.writeLabeledInt(&b, "tables:", d.src.TableCount)
	d.writeLabeledInt(&b, "views:", d.src.ViewCount)
	if d.src.Size != nil && *d.src.Size > 0 {
		fmt.Fprintf(&b, "%s %s bytes\n",
			d.theme.Faint.Render("size:"),
			d.theme.Number.Render(strconv.FormatInt(*d.src.Size, 10)))
	}
	return b.String()
}

// writeLabeledInt writes "<faint-label> <cyan-int>\n" to b.
func (d *detailPane) writeLabeledInt(b *strings.Builder, label string, n int64) {
	fmt.Fprintf(b, "%s %s\n",
		d.theme.Faint.Render(label),
		d.theme.Number.Render(strconv.FormatInt(n, 10)))
}

func (d *detailPane) viewTable() string {
	if d.tbl == nil {
		return d.theme.Faint.Render("(loading)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(d.tbl.Name))
	if d.tbl.TableType != "" {
		fmt.Fprintf(&b, "%s %s\n", d.theme.Faint.Render("type:"), d.tbl.TableType)
	}
	d.writeLabeledInt(&b, "rows:", d.tbl.RowCount)
	if d.tbl.Size != nil {
		fmt.Fprintf(&b, "%s %s bytes\n",
			d.theme.Faint.Render("size:"),
			d.theme.Number.Render(strconv.FormatInt(*d.tbl.Size, 10)))
	}
	b.WriteString("\n")

	if len(d.tbl.Columns) > 0 {
		fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(fmt.Sprintf("columns (%d)", len(d.tbl.Columns))))
		for i, c := range d.tbl.Columns {
			flag := ""
			if c.PrimaryKey {
				flag = " PK"
			}
			if !c.Nullable {
				flag += " NOT NULL"
			}
			fmt.Fprintf(&b, "%s %-20s %s%s\n",
				d.theme.Faint.Render(fmt.Sprintf("%2d", i+1)),
				c.Name,
				d.theme.Faint.Render(c.BaseType),
				d.theme.Faint.Render(flag),
			)
		}
		b.WriteString("\n")
	}
	if len(d.tbl.Indexes) > 0 {
		fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(fmt.Sprintf("indexes (%d)", len(d.tbl.Indexes))))
		for _, idx := range d.tbl.Indexes {
			kind := "idx"
			switch {
			case idx.Primary:
				kind = "pk"
			case idx.Unique:
				kind = "uniq"
			}
			fmt.Fprintf(&b, "  %-6s %s (%s)\n", kind, idx.Name, strings.Join(idx.Columns, ", "))
		}
		b.WriteString("\n")
	}
	if d.tbl.FK != nil && (len(d.tbl.FK.Outgoing) > 0 || len(d.tbl.FK.Incoming) > 0) {
		hdr := fmt.Sprintf("fk (%d↗ %d↙)", len(d.tbl.FK.Outgoing), len(d.tbl.FK.Incoming))
		fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(hdr))
		for _, fk := range d.tbl.FK.Outgoing {
			fmt.Fprintf(&b, "  ↗ %s.%s → %s.%s\n",
				fk.Table, strings.Join(fk.Columns, ","),
				fk.RefTable, strings.Join(fk.RefColumns, ","))
		}
		for _, fk := range d.tbl.FK.Incoming {
			fmt.Fprintf(&b, "  ↙ %s.%s → %s.%s\n",
				fk.Table, strings.Join(fk.Columns, ","),
				fk.RefTable, strings.Join(fk.RefColumns, ","))
		}
		b.WriteString("\n")
	}
	if len(d.tbl.UniqueConstraints) > 0 {
		fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(fmt.Sprintf("unique (%d)", len(d.tbl.UniqueConstraints))))
		for _, uc := range d.tbl.UniqueConstraints {
			fmt.Fprintf(&b, "  %s (%s)\n", uc.Name, strings.Join(uc.Columns, ", "))
		}
	}
	if d.preview != nil {
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(fmt.Sprintf("preview (%d rows)", len(d.preview.rows))))
		switch {
		case d.preview.err != nil:
			b.WriteString(d.theme.Error.Render(d.preview.err.Error()))
			b.WriteString("\n")
		case len(d.preview.rows) == 0 && !d.preview.done:
			b.WriteString(d.theme.Faint.Render("(loading)"))
			b.WriteString("\n")
		default:
			for i, r := range d.preview.rows {
				if i >= 10 {
					fmt.Fprintf(&b, "… %d more\n", len(d.preview.rows)-10)
					break
				}
				b.WriteString(formatRecord(r))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// formatRecord renders a single record as a pipe-separated row. Long
// values are truncated at 30 chars.
func formatRecord(r record.Record) string {
	parts := make([]string, len(r))
	for i, v := range r {
		s := fmt.Sprintf("%v", v)
		// Truncate by rune, not byte, so multi-byte UTF-8 isn't split.
		if rs := []rune(s); len(rs) > 30 {
			s = string(rs[:27]) + "…"
		}
		parts[i] = s
	}
	return strings.Join(parts, " │ ")
}

func (d *detailPane) viewColumn() string {
	if d.col == nil {
		return d.theme.Faint.Render("(loading)")
	}
	c := d.col
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(c.Name))
	fmt.Fprintf(&b, "position:    %d\n", c.Position)
	fmt.Fprintf(&b, "base type:   %s\n", c.BaseType)
	fmt.Fprintf(&b, "column type: %s\n", c.ColumnType)
	fmt.Fprintf(&b, "kind:        %s\n", c.Kind)
	fmt.Fprintf(&b, "nullable:    %v\n", c.Nullable)
	if c.PrimaryKey {
		b.WriteString("primary key: yes\n")
	}
	if c.DefaultValue != "" {
		fmt.Fprintf(&b, "default:     %s\n", c.DefaultValue)
	}
	if c.Comment != "" {
		fmt.Fprintf(&b, "comment:     %s\n", c.Comment)
	}
	return b.String()
}
