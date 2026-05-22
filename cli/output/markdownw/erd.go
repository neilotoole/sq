package markdownw

import (
	"bytes"
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// erdEdge is a single relationship line in a Mermaid erDiagram. The
// parent is the referenced (PK) side and the child is the referencing
// (FK) side. card is the Mermaid cardinality token, e.g. "||--o{".
type erdEdge struct {
	parent string
	child  string
	card   string
	label  string
}

// writeSourceERD writes the whole-source entity-relationship diagram:
// an entity block for every table that has columns, plus an edge for
// every in-source outgoing foreign key.
func (w *metadataWriter) writeSourceERD(buf *bytes.Buffer, tables []*metadata.Table) {
	byName := tableIndex(tables)

	var edges []erdEdge
	for _, tbl := range tables {
		if tbl.FK == nil {
			continue
		}
		for _, fk := range tbl.FK.Outgoing {
			if e, ok := fkEdge(fk, byName); ok {
				edges = append(edges, e)
			}
		}
	}

	w.renderERD(buf, tables, sortDedupEdges(edges), 2)
}

// writeTableERD writes a focused entity-relationship diagram for a
// single table: the table's own entity block (its columns) plus an edge
// for every foreign key it participates in. Related tables are rendered
// by Mermaid as bare, column-less entity boxes — the edges point at just
// the neighbor's name, not a full diagram of it.
//
// headingLevel sets the Markdown '#' level of the diagram's heading: 2
// for a single-table inspect (under a level-1 table heading) and 4 for a
// per-table diagram nested under a whole-source inspect's level-3 table
// heading. cardIndex, when non-nil, supplies neighbor tables for
// cardinality inference (whole-source inspect, where every table is
// known), keeping per-table cardinality consistent with the whole-source
// diagram; pass nil for a single-table inspect, where only tbl is known
// and the neighbor side falls back to the default cardinality.
func (w *metadataWriter) writeTableERD(
	buf *bytes.Buffer, tbl *metadata.Table, headingLevel int, cardIndex map[string]*metadata.Table,
) {
	if cardIndex == nil {
		cardIndex = map[string]*metadata.Table{tbl.Name: tbl}
	}

	var edges []erdEdge
	if tbl.FK != nil {
		for _, fk := range tbl.FK.Outgoing {
			if e, ok := fkEdge(fk, cardIndex); ok {
				edges = append(edges, e)
			}
		}
		for _, fk := range tbl.FK.Incoming {
			if e, ok := fkEdge(fk, cardIndex); ok {
				edges = append(edges, e)
			}
		}
	}

	w.renderERD(buf, []*metadata.Table{tbl}, sortDedupEdges(edges), headingLevel)
}

// renderERD writes the "Entity Relationship Diagram" section at the given
// Markdown heading level: a fenced ```mermaid erDiagram with an entity
// block per table in entities that has columns, followed by edges. It
// writes nothing when there is neither an entity block nor an edge to
// draw.
func (w *metadataWriter) renderERD(
	buf *bytes.Buffer, entities []*metadata.Table, edges []erdEdge, headingLevel int,
) {
	var withCols []*metadata.Table
	for _, t := range entities {
		if len(t.Columns) > 0 {
			withCols = append(withCols, t)
		}
	}
	if len(withCols) == 0 && len(edges) == 0 {
		return
	}

	fmt.Fprintf(buf, "\n%s Entity Relationship Diagram\n\n```mermaid\nerDiagram\n",
		strings.Repeat("#", headingLevel))

	for _, t := range withCols {
		fkCols := fkColumnSet(t)
		fmt.Fprintf(buf, "    %s {\n", mermaidIdent(t.Name))
		for _, col := range t.Columns {
			typ := mermaidAttrWord(col.Kind.String())
			name := mermaidAttrWord(col.Name)
			if keys := mermaidKeys(col, fkCols); keys != "" {
				fmt.Fprintf(buf, "        %s %s %s\n", typ, name, keys)
			} else {
				fmt.Fprintf(buf, "        %s %s\n", typ, name)
			}
		}
		buf.WriteString("    }\n")
	}

	for _, e := range edges {
		fmt.Fprintf(buf, "    %s %s %s : %s\n",
			mermaidIdent(e.parent), e.card, mermaidIdent(e.child), e.label)
	}

	buf.WriteString("```\n")
}

// fkEdge builds the erdEdge for a single foreign key. It returns false
// for references that point outside this source (a non-empty RefCatalog
// or RefSchema after [metadata.LinkForeignKeys] normalization), which
// are described in the table's prose but omitted from the diagram.
//
// Cardinality is inferred from the child (referencing) table when it's
// available in byName: the parent side is "|o" (zero-or-one) when any FK
// column is nullable, else "||" (exactly one); the child side is "||"
// (one-to-one) when the FK columns are themselves unique on the child,
// else "o{" (zero-or-many). When the child table isn't available (the
// neighbor side of a single-table inspect) the default "||--o{" is used.
func fkEdge(fk *metadata.ForeignKey, byName map[string]*metadata.Table) (erdEdge, bool) {
	if fk == nil || fk.RefCatalog != "" || fk.RefSchema != "" {
		return erdEdge{}, false
	}

	parentCard, childCard := "||", "o{"
	if child := byName[fk.Table]; child != nil {
		if fkColumnsNullable(child, fk.Columns) {
			parentCard = "|o"
		}
		if fkColumnsUnique(child, fk.Columns) {
			childCard = "||"
		}
	}

	label := `""`
	if fk.Name != "" {
		label = `"` + strings.ReplaceAll(fk.Name, `"`, "") + `"`
	}

	return erdEdge{
		parent: fk.RefTable,
		child:  fk.Table,
		card:   parentCard + "--" + childCard,
		label:  label,
	}, true
}

// fkColumnsNullable reports whether any of the named columns on tbl is
// nullable.
func fkColumnsNullable(tbl *metadata.Table, cols []string) bool {
	for _, name := range cols {
		if c := tbl.Column(name); c != nil && c.Nullable {
			return true
		}
	}
	return false
}

// fkColumnsUnique reports whether cols form the primary key or a unique
// constraint on tbl (i.e. the referencing side is itself unique, making
// the relationship one-to-one).
func fkColumnsUnique(tbl *metadata.Table, cols []string) bool {
	var pk []string
	for _, c := range tbl.Columns {
		if c.PrimaryKey {
			pk = append(pk, c.Name)
		}
	}
	if sameColSet(pk, cols) {
		return true
	}
	for _, uc := range tbl.UniqueConstraints {
		if uc != nil && sameColSet(uc.Columns, cols) {
			return true
		}
	}
	return false
}

// sameColSet reports whether a and b contain the same column names,
// ignoring order. An empty set never matches.
func sameColSet(a, b []string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	slices.Sort(aa)
	slices.Sort(bb)
	return slices.Equal(aa, bb)
}

// mermaidKeys returns the "PK", "FK", or "PK,FK" attribute marker for a
// column in a Mermaid entity block, or "" when the column is neither.
func mermaidKeys(col *metadata.Column, fkCols map[string]bool) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if fkCols[col.Name] {
		parts = append(parts, "FK")
	}
	return strings.Join(parts, ",")
}

// tableIndex returns a name→table lookup for tables.
func tableIndex(tables []*metadata.Table) map[string]*metadata.Table {
	byName := make(map[string]*metadata.Table, len(tables))
	for _, t := range tables {
		byName[t.Name] = t
	}
	return byName
}

// sortDedupEdges sorts edges into a deterministic order and removes
// exact duplicates.
func sortDedupEdges(edges []erdEdge) []erdEdge {
	slices.SortFunc(edges, func(a, b erdEdge) int {
		if c := cmp.Compare(a.child, b.child); c != 0 {
			return c
		}
		if c := cmp.Compare(a.parent, b.parent); c != 0 {
			return c
		}
		if c := cmp.Compare(a.card, b.card); c != 0 {
			return c
		}
		return cmp.Compare(a.label, b.label)
	})
	return slices.CompactFunc(edges, func(a, b erdEdge) bool { return a == b })
}

// mermaidIdentRe matches identifiers safe to emit unquoted in a Mermaid
// diagram.
var mermaidIdentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// mermaidIdent renders an entity (table) name for a Mermaid erDiagram,
// quoting it when it contains characters Mermaid wouldn't accept bare.
// Mermaid supports double-quoted entity names (in both entity blocks and
// relationship endpoints), so quoting preserves the real name.
func mermaidIdent(s string) string {
	if mermaidIdentRe.MatchString(s) {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, "") + `"`
}

// mermaidAttrWordRe matches characters not allowed in a Mermaid
// erDiagram attribute type or name token.
var mermaidAttrWordRe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// mermaidAttrWord sanitizes s into a Mermaid attribute word. Unlike
// entity names, attribute type/name tokens cannot be quoted — Mermaid
// parses a quoted token there as a trailing comment — so any character
// outside the safe set is replaced with an underscore (e.g. a column
// literally named "zip code" becomes zip_code). The column's true name
// is preserved in the Markdown column table.
func mermaidAttrWord(s string) string {
	if s == "" {
		return "_"
	}
	return mermaidAttrWordRe.ReplaceAllString(s, "_")
}
