// Package mermaid generates Mermaid.js erDiagram source from sq table
// metadata. It returns the bare diagram source (no Markdown code fence or
// HTML wrapper); callers wrap it for their output format.
package mermaid

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/source/metadata"
)

// erdEdge is a single relationship line in a Mermaid erDiagram. parent is
// the referenced (PK) side, child the referencing (FK) side, card the
// Mermaid cardinality token (e.g. "||--o{").
type erdEdge struct {
	parent string
	child  string
	card   string
	label  string
}

// SourceDiagram returns the whole-source erDiagram source: an entity block
// for every table that has columns, plus an edge for every in-source
// outgoing foreign key. Returns "" when there is nothing to draw.
func SourceDiagram(tables []*metadata.Table) string {
	byName := Index(tables)
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
	return render(tables, sortDedupEdges(edges))
}

// TableDiagram returns a focused erDiagram source for tbl: tbl's own entity
// block (its columns) plus an edge for every foreign key it participates
// in. Related tables are rendered as bare, column-less entity boxes — the
// edges point at just the neighbor's name. cardIndex, when non-nil,
// supplies neighbor tables for cardinality inference (pass Index(tables) in
// a whole-source context); pass nil for a single-table inspect, where only
// tbl is known. Returns "" when there is nothing to draw.
func TableDiagram(tbl *metadata.Table, cardIndex map[string]*metadata.Table) string {
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
	return render([]*metadata.Table{tbl}, sortDedupEdges(edges))
}

// Index returns a name→table lookup, for use as TableDiagram's cardIndex.
func Index(tables []*metadata.Table) map[string]*metadata.Table {
	byName := make(map[string]*metadata.Table, len(tables))
	for _, t := range tables {
		byName[t.Name] = t
	}
	return byName
}

// render builds the erDiagram source: an entity block per table in entities
// that has columns, followed by edges. Returns "" when there is neither an
// entity block nor an edge to draw.
func render(entities []*metadata.Table, edges []erdEdge) string {
	var withCols []*metadata.Table
	for _, t := range entities {
		if len(t.Columns) > 0 {
			withCols = append(withCols, t)
		}
	}
	if len(withCols) == 0 && len(edges) == 0 {
		return ""
	}

	buf := &strings.Builder{}
	buf.WriteString("erDiagram\n")
	for _, t := range withCols {
		fkCols := fkColumnSet(t)
		fmt.Fprintf(buf, "    %s {\n", ident(t.Name))
		for _, col := range t.Columns {
			typ := attrWord(col.Kind.String())
			name := attrWord(col.Name)
			if keys := keyMarker(col, fkCols); keys != "" {
				fmt.Fprintf(buf, "        %s %s %s\n", typ, name, keys)
			} else {
				fmt.Fprintf(buf, "        %s %s\n", typ, name)
			}
		}
		buf.WriteString("    }\n")
	}
	for _, e := range edges {
		fmt.Fprintf(buf, "    %s %s %s : %s\n", ident(e.parent), e.card, ident(e.child), e.label)
	}
	return buf.String()
}

// fkEdge builds the erdEdge for a single foreign key. Returns false for
// references that point outside this source (non-empty RefCatalog/RefSchema
// after metadata.LinkForeignKeys normalization).
//
// Cardinality is inferred from the child (referencing) table when it's
// available in byName: the parent side is "|o" (zero-or-one) when any FK
// column is nullable, else "||" (exactly one); the child side is "||"
// (one-to-one) when the FK columns are themselves unique on the child,
// else "o{" (zero-or-many). When the child table isn't available the
// default "||--o{" is used.
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
		label = `"` + mermaidQuoteSafe.Replace(fk.Name) + `"`
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

// keyMarker returns the "PK", "FK", or "PK,FK" attribute marker for a
// column in a Mermaid entity block, or "" when the column is neither.
func keyMarker(col *metadata.Column, fkCols map[string]bool) string {
	var parts []string
	if col.PrimaryKey {
		parts = append(parts, "PK")
	}
	if fkCols[col.Name] {
		parts = append(parts, "FK")
	}
	return strings.Join(parts, ",")
}

// fkColumnSet returns the set of column names on tbl that participate in any
// outgoing foreign key.
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

// sortDedupEdges sorts edges into a deterministic order and removes exact
// duplicates.
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

// identRe matches identifiers safe to emit unquoted in a Mermaid diagram.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// mermaidQuoteSafe strips characters that would corrupt a double-quoted
// Mermaid token: a literal double-quote terminates the string, and a newline,
// carriage return, or tab would split or break the diagram line.
var mermaidQuoteSafe = strings.NewReplacer(`"`, "", "\n", " ", "\r", " ", "\t", " ")

// ident renders an entity (table) name, quoting it when it contains
// characters Mermaid wouldn't accept bare.
func ident(s string) string {
	if identRe.MatchString(s) {
		return s
	}
	return `"` + mermaidQuoteSafe.Replace(s) + `"`
}

// attrWordRe matches characters not allowed in a Mermaid erDiagram
// attribute type or name token.
var attrWordRe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// attrWord sanitizes s into a Mermaid attribute word. A quoted token in an
// attribute position is parsed by Mermaid as a comment, so characters
// outside the safe set are replaced with underscores (e.g. "zip code" →
// zip_code).
func attrWord(s string) string {
	if s == "" {
		return "_"
	}
	return attrWordRe.ReplaceAllString(s, "_")
}
