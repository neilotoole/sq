package explore

import (
	"fmt"
	"strings"

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
	src   *metadata.Source
	tbl   *metadata.Table
	col   *metadata.Column
	theme theme
	kind  detailKind
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
	return style.Width(width).Height(height).Render(body)
}

func (d *detailPane) viewSource() string {
	if d.src == nil {
		return d.theme.Faint.Render("(loading)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(d.src.Handle))
	if d.src.DBProduct != "" {
		fmt.Fprintf(&b, "driver:    %s\n", d.src.DBProduct)
	} else if d.src.Driver != "" {
		fmt.Fprintf(&b, "driver:    %s\n", d.src.Driver)
	}
	if d.src.Location != "" {
		fmt.Fprintf(&b, "location:  %s\n", d.src.Location)
	}
	fmt.Fprintf(&b, "tables: %d\n", d.src.TableCount)
	fmt.Fprintf(&b, "views: %d\n", d.src.ViewCount)
	if d.src.Size > 0 {
		fmt.Fprintf(&b, "size:   %d bytes\n", d.src.Size)
	}
	return b.String()
}

func (d *detailPane) viewTable() string {
	if d.tbl == nil {
		return d.theme.Faint.Render("(loading)")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", d.theme.Title.Render(d.tbl.Name))
	if d.tbl.TableType != "" {
		fmt.Fprintf(&b, "type:  %s\n", d.tbl.TableType)
	}
	fmt.Fprintf(&b, "rows: %d\n", d.tbl.RowCount)
	if d.tbl.Size != nil {
		fmt.Fprintf(&b, "size:  %d bytes\n", *d.tbl.Size)
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
			fmt.Fprintf(&b, "%2d %-20s %s%s\n", i+1, c.Name, c.BaseType, flag)
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
	return b.String()
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
