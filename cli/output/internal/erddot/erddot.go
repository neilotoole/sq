// Package erddot generates Graphviz DOT source describing an
// entity-relationship diagram from sq table metadata. It returns the bare
// DOT source (a "digraph") for a renderer such as go-graphviz to rasterize
// to SVG/PNG; it does not itself render an image.
//
// erddot is the Graphviz counterpart to package mermaid: both consume the
// same schema metadata and share foreign-key cardinality inference via
// package erdmodel, so the relationships they draw agree for a given
// schema. Entities are rendered as Graphviz HTML-like table labels (one row
// per column, with the column's type and PK/FK markers); foreign keys are
// rendered as edges with crow's-foot arrowheads.
package erddot

import (
	"cmp"
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/neilotoole/sq/cli/output/internal/erdmodel"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// SourceDiagram returns the whole-source DOT diagram: an entity node for
// every table that has columns, plus an edge for every in-source outgoing
// foreign key. Returns "" when there is nothing to draw.
func SourceDiagram(tables []*metadata.Table) string {
	byName := erdmodel.Index(tables)
	var edges []erdmodel.Edge
	for _, tbl := range tables {
		if tbl.FK == nil {
			continue
		}
		for _, fk := range tbl.FK.Outgoing {
			if e, ok := erdmodel.Resolve(fk, byName); ok {
				edges = append(edges, e)
			}
		}
	}
	return render(tables, sortDedupEdges(edges))
}

// TableDiagram returns a focused DOT diagram for tbl: tbl's own entity node
// (its columns) plus an edge for every foreign key it participates in.
// Related tables are rendered as bare, column-less nodes — the edges point
// at just the neighbor's name. cardIndex, when non-nil, supplies neighbor
// tables for cardinality inference (pass Index(tables) in a whole-source
// context); pass nil for a single-table inspect, where only tbl is known.
// Returns "" when there is nothing to draw.
func TableDiagram(tbl *metadata.Table, cardIndex map[string]*metadata.Table) string {
	if cardIndex == nil {
		cardIndex = map[string]*metadata.Table{tbl.Name: tbl}
	}
	var edges []erdmodel.Edge
	if tbl.FK != nil {
		for _, fk := range tbl.FK.Outgoing {
			if e, ok := erdmodel.Resolve(fk, cardIndex); ok {
				edges = append(edges, e)
			}
		}
		for _, fk := range tbl.FK.Incoming {
			if e, ok := erdmodel.Resolve(fk, cardIndex); ok {
				edges = append(edges, e)
			}
		}
	}
	return render([]*metadata.Table{tbl}, sortDedupEdges(edges))
}

// Index returns a name→table lookup, for use as TableDiagram's cardIndex.
func Index(tables []*metadata.Table) map[string]*metadata.Table {
	return erdmodel.Index(tables)
}

// render builds the DOT source: an entity node per table in entities that
// has columns, followed by edges. Returns "" when there is neither an entity
// node nor an edge to draw.
func render(entities []*metadata.Table, edges []erdmodel.Edge) string {
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
	buf.WriteString("digraph erd {\n")
	buf.WriteString("    rankdir=LR;\n")
	buf.WriteString(`    node [shape=plaintext, fontname="Helvetica"];` + "\n")
	buf.WriteString(`    edge [fontname="Helvetica", fontsize=10];` + "\n")
	for _, t := range withCols {
		writeEntity(buf, t)
	}
	for _, e := range edges {
		writeEdge(buf, e)
	}
	buf.WriteString("}\n")
	return buf.String()
}

// writeEntity writes a single table as a Graphviz HTML-like table label: a
// header row with the table name, then one row per column showing the column
// name and its type with any PK/FK marker. The label is emitted across
// multiple lines (Graphviz ignores whitespace between HTML-like elements) so
// the generated DOT stays readable.
func writeEntity(buf *strings.Builder, t *metadata.Table) {
	fkCols := erdmodel.FKColumnSet(t)
	fmt.Fprintf(buf, "    %s [label=<\n", quoteID(t.Name))
	buf.WriteString("        <table border=\"0\" cellborder=\"1\" cellspacing=\"0\" cellpadding=\"4\">\n")
	fmt.Fprintf(buf,
		"            <tr><td bgcolor=\"#e8e8e8\" colspan=\"2\"><b>%s</b></td></tr>\n",
		escapeLabel(t.Name))
	for _, col := range t.Columns {
		typ := escapeLabel(col.ColumnType)
		if marker := erdmodel.KeyMarker(col, fkCols); marker != "" {
			typ += " " + marker
		}
		fmt.Fprintf(buf,
			"            <tr><td align=\"left\">%s</td><td align=\"left\">%s</td></tr>\n",
			escapeLabel(col.Name), typ)
	}
	buf.WriteString("        </table>\n")
	buf.WriteString("    >];\n")
}

// writeEdge writes a single foreign-key relationship as a DOT edge from the
// parent (referenced/PK) table to the child (referencing/FK) table, with
// crow's-foot arrowheads encoding the cardinality and the constraint name
// as the edge label.
func writeEdge(buf *strings.Builder, e erdmodel.Edge) {
	tail, head := arrowTokens(e.Card)
	fmt.Fprintf(buf, "    %s -> %s [dir=both, arrowtail=%s, arrowhead=%s",
		quoteID(e.Parent), quoteID(e.Child), tail, head)
	if e.Label != "" {
		fmt.Fprintf(buf, ", label=%s", quoteID(e.Label))
	}
	buf.WriteString("];\n")
}

// arrowTokens maps a renderer-neutral cardinality to Graphviz crow's-foot
// arrow types. tail is drawn at the parent (referenced/PK) end, head at the
// child (referencing/FK) end:
//
//   - parent: "teeodot" (zero-or-one) when optional, else "tee" (exactly one)
//   - child: "tee" (one-to-one) when unique, else "crowodot" (zero-or-many)
func arrowTokens(c erdmodel.Cardinality) (tail, head string) {
	tail = "tee"
	if c.ParentOptional {
		tail = "teeodot"
	}
	head = "crowodot"
	if c.ChildUnique {
		head = "tee"
	}
	return tail, head
}

// sortDedupEdges sorts edges into a deterministic order and removes exact
// duplicates.
func sortDedupEdges(edges []erdmodel.Edge) []erdmodel.Edge {
	slices.SortFunc(edges, func(a, b erdmodel.Edge) int {
		if c := cmp.Compare(a.Child, b.Child); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Parent, b.Parent); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Label, b.Label); c != 0 {
			return c
		}
		if c := cmp.Compare(boolRank(a.Card.ParentOptional), boolRank(b.Card.ParentOptional)); c != 0 {
			return c
		}
		return cmp.Compare(boolRank(a.Card.ChildUnique), boolRank(b.Card.ChildUnique))
	})
	return slices.CompactFunc(edges, func(a, b erdmodel.Edge) bool { return a == b })
}

// boolRank maps false→0, true→1 for deterministic edge ordering.
func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ctrlSafe neutralizes control characters that would split a DOT statement
// or HTML-like label across lines (newline, carriage return, tab → space).
// It is applied before both quoteID and escapeLabel so a column name used as
// an HTML-like label cell and a quoted edge endpoint stays consistent.
var ctrlSafe = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ")

// dotIDEscape escapes the characters that are special inside a DOT
// double-quoted ID: a backslash (the escape character) and a double quote
// (which would terminate the string).
var dotIDEscape = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// quoteID renders s as a DOT double-quoted identifier, used for node IDs and
// edge endpoints. Quoting unconditionally lets any table name (spaces,
// punctuation) be emitted safely.
func quoteID(s string) string {
	return `"` + dotIDEscape.Replace(ctrlSafe.Replace(s)) + `"`
}

// escapeLabel renders s as text inside a Graphviz HTML-like label, escaping
// the XML metacharacters (&, <, >, ", '). Graphviz unescapes these when it
// parses the label, so the displayed text matches the original.
func escapeLabel(s string) string {
	return html.EscapeString(ctrlSafe.Replace(s))
}
