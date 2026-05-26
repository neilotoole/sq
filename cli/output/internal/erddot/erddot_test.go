package erddot_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output/internal/erddot"
	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// testTables builds the same deterministic two-table source (actor +
// film_actor, with film_actor.actor_id → actor.actor_id) used by the
// mermaid package tests, and links its foreign keys so FK.Incoming is
// populated.
func testTables() []*metadata.Table {
	actor := &metadata.Table{
		Name: "actor", TableType: "table", RowCount: 200,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "first_name", Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}
	filmActor := &metadata.Table{
		Name: "film_actor", TableType: "table", RowCount: 5462,
		Columns: []*metadata.Column{
			{Name: "actor_id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "film_id", Position: 2, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk_film_actor_actor", Table: "film_actor", Columns: []string{"actor_id"},
			RefTable: "actor", RefColumns: []string{"actor_id"},
		}}},
	}
	src := &metadata.Source{Handle: "@test", Tables: []*metadata.Table{actor, filmActor}}
	metadata.LinkForeignKeys(nil, src)
	return src.Tables
}

func TestSourceDiagram(t *testing.T) {
	got := erddot.SourceDiagram(testTables())
	require.Equal(t, `digraph erd {
    rankdir=LR;
    node [shape=plaintext, fontname="Helvetica"];
    edge [fontname="Helvetica", fontsize=10];
    "actor" [label=<
        <table border="0" cellborder="1" cellspacing="0" cellpadding="4">
            <tr><td bgcolor="#e8e8e8" colspan="2"><b>actor</b></td></tr>
            <tr><td align="left">actor_id</td><td align="left">INTEGER PK</td></tr>
            <tr><td align="left">first_name</td><td align="left">TEXT</td></tr>
        </table>
    >];
    "film_actor" [label=<
        <table border="0" cellborder="1" cellspacing="0" cellpadding="4">
            <tr><td bgcolor="#e8e8e8" colspan="2"><b>film_actor</b></td></tr>
            <tr><td align="left">actor_id</td><td align="left">INTEGER PK,FK</td></tr>
            <tr><td align="left">film_id</td><td align="left">INTEGER PK</td></tr>
        </table>
    >];
    "actor" -> "film_actor" [dir=both, arrowtail=tee, arrowhead=crowodot, label="fk_film_actor_actor"];
}
`, got)
}

func TestTableDiagram_focused(t *testing.T) {
	tables := testTables()
	got := erddot.TableDiagram(tables[1], erddot.Index(tables)) // film_actor
	require.Equal(t, `digraph erd {
    rankdir=LR;
    node [shape=plaintext, fontname="Helvetica"];
    edge [fontname="Helvetica", fontsize=10];
    "film_actor" [label=<
        <table border="0" cellborder="1" cellspacing="0" cellpadding="4">
            <tr><td bgcolor="#e8e8e8" colspan="2"><b>film_actor</b></td></tr>
            <tr><td align="left">actor_id</td><td align="left">INTEGER PK,FK</td></tr>
            <tr><td align="left">film_id</td><td align="left">INTEGER PK</td></tr>
        </table>
    >];
    "actor" -> "film_actor" [dir=both, arrowtail=tee, arrowhead=crowodot, label="fk_film_actor_actor"];
}
`, got)
}

func TestSourceDiagram_empty(t *testing.T) {
	require.Equal(t, "", erddot.SourceDiagram(nil))

	// A column-less table has nothing to draw.
	src := []*metadata.Table{{Name: "t", TableType: "table"}}
	require.Equal(t, "", erddot.SourceDiagram(src))
}

// TestSourceDiagram_escaping verifies that table/column names with spaces or
// XML metacharacters are emitted safely: node IDs and edge endpoints are
// double-quoted (DOT-escaped), and HTML-like label text is XML-escaped.
func TestSourceDiagram_escaping(t *testing.T) {
	tables := []*metadata.Table{{
		Name: `weird"table`, TableType: "table",
		Columns: []*metadata.Column{
			{Name: "zip code", Position: 1, PrimaryKey: true, ColumnType: "TEXT", Kind: kind.Text},
			{Name: `a<b>&c`, Position: 2, ColumnType: "TEXT", Kind: kind.Text},
		},
	}}
	got := erddot.SourceDiagram(tables)

	// Node ID is double-quoted, with the embedded quote DOT-escaped.
	require.Contains(t, got, `"weird\"table" [label=<`)
	// Spaces in label text need no escaping.
	require.Contains(t, got, `<td align="left">zip code</td>`)
	// XML metacharacters in label text are escaped.
	require.Contains(t, got, `<td align="left">a&lt;b&gt;&amp;c</td>`)
	require.NotContains(t, got, "a<b>&c")
}

// TestSourceDiagram_neutralizesControlChars verifies that a newline or tab in
// a name is replaced with a space, so the emitted DOT stays one statement per
// line.
func TestSourceDiagram_neutralizesControlChars(t *testing.T) {
	parent := &metadata.Table{
		Name: "parent", TableType: "table",
		Columns: []*metadata.Column{
			{Name: "id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
		},
	}
	child := &metadata.Table{
		Name: "ch\nild", TableType: "table",
		Columns: []*metadata.Column{
			{Name: "id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
			{Name: "parent_id", Position: 2, ColumnType: "INTEGER", Kind: kind.Int},
		},
		FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
			Name: "fk\tparent", Table: "ch\nild", Columns: []string{"parent_id"},
			RefTable: "parent", RefColumns: []string{"id"},
		}}},
	}
	src := &metadata.Source{Handle: "@q", Tables: []*metadata.Table{parent, child}}
	metadata.LinkForeignKeys(nil, src)
	got := erddot.SourceDiagram(src.Tables)

	require.Contains(t, got, `"ch ild" [label=<`)
	require.Contains(t, got, `label="fk parent"`)
	require.NotContains(t, got, "ch\nild", "raw newline must not leak into the diagram")
	require.NotContains(t, got, "fk\tparent", "raw tab must not leak into the diagram")
}

// TestCardinalityParity verifies that the DOT renderer and the Mermaid
// renderer agree on cardinality for a range of FK shapes: they share
// erdmodel.Resolve, so an optional/unique relationship that Mermaid draws
// as "|o" / "||" must come out as the corresponding "teeodot" / "tee"
// arrowhead in DOT, and a plain one-to-many ("||--o{") as "tee"/"crowodot".
func TestCardinalityParity(t *testing.T) {
	testCases := []struct {
		name        string
		childPK     bool // FK column is the child's primary key (=> unique => one-to-one)
		fkNullable  bool // FK column is nullable (=> parent optional)
		wantMermaid string
		wantTail    string // DOT arrowtail (parent side)
		wantHead    string // DOT arrowhead (child side)
	}{
		{
			name: "one_to_many", childPK: false, fkNullable: false,
			wantMermaid: "||--o{", wantTail: "tee", wantHead: "crowodot",
		},
		{
			name: "optional_one_to_many", childPK: false, fkNullable: true,
			wantMermaid: "|o--o{", wantTail: "teeodot", wantHead: "crowodot",
		},
		{
			name: "one_to_one", childPK: true, fkNullable: false,
			wantMermaid: "||--||", wantTail: "tee", wantHead: "tee",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parent := &metadata.Table{
				Name: "parent", TableType: "table",
				Columns: []*metadata.Column{
					{Name: "id", Position: 1, PrimaryKey: true, ColumnType: "INTEGER", Kind: kind.Int},
				},
			}
			child := &metadata.Table{
				Name: "child", TableType: "table",
				Columns: []*metadata.Column{
					{
						Name: "parent_id", Position: 1, PrimaryKey: tc.childPK,
						Nullable: tc.fkNullable, ColumnType: "INTEGER", Kind: kind.Int,
					},
				},
				FK: &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{
					Name: "fk_child_parent", Table: "child", Columns: []string{"parent_id"},
					RefTable: "parent", RefColumns: []string{"id"},
				}}},
			}
			src := &metadata.Source{Handle: "@p", Tables: []*metadata.Table{parent, child}}
			metadata.LinkForeignKeys(nil, src)

			gotMermaid := mermaid.SourceDiagram(src.Tables)
			require.Contains(t, gotMermaid, "parent "+tc.wantMermaid+" child",
				"mermaid cardinality token")

			gotDOT := erddot.SourceDiagram(src.Tables)
			require.Contains(t, gotDOT,
				`"parent" -> "child" [dir=both, arrowtail=`+tc.wantTail+`, arrowhead=`+tc.wantHead+`,`,
				"DOT arrowheads must match the Mermaid cardinality")
		})
	}
}
