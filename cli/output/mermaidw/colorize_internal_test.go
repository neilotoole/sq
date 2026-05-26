package mermaidw

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
)

// colorPrinting returns a color-enabled *output.Printing for the colorizer
// unit tests.
func colorPrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(true)
	return pr
}

// TestColorize_passthrough confirms colorize returns its input unchanged when
// there's no color config (nil) or in monochrome mode — the two states in
// which non-TTY output must stay byte-identical to the plain diagram.
func TestColorize_passthrough(t *testing.T) {
	const src = "erDiagram\n    actor {\n        int id PK\n    }\n"

	require.Equal(t, src, colorize(src, nil), "nil Printing")

	mono := output.NewPrinting()
	mono.EnableColor(false)
	require.Equal(t, src, colorize(src, mono), "monochrome Printing")
}

// TestColorizeLine covers each line-grammar branch directly, including the
// parity branches mirrored from #691 that sq's own renderer never emits
// (comments, label-less relationships) and the safe passthrough for an
// unrecognized line.
func TestColorizeLine(t *testing.T) {
	pr := colorPrinting()

	testCases := map[string]struct {
		in   string
		want string
	}{
		"blank": {
			in:   "",
			want: "",
		},
		"indent_only": {
			in:   "    ",
			want: "    ",
		},
		"keyword": {
			in:   "erDiagram",
			want: pr.Key.Sprint("erDiagram"),
		},
		"entity_open": {
			in:   "    actor {",
			want: "    " + pr.Header.Sprint("actor") + " " + pr.Punc.Sprint("{"),
		},
		"entity_open_quoted": {
			in:   `    "weird table" {`,
			want: "    " + pr.Header.Sprint(`"weird table"`) + " " + pr.Punc.Sprint("{"),
		},
		"entity_close": {
			in:   "    }",
			want: "    " + pr.Punc.Sprint("}"),
		},
		"attr_no_keys": {
			in:   "        text first_name",
			want: "        " + pr.Number.Sprint("text") + " first_name",
		},
		"attr_keys": {
			in:   "        int actor_id PK,FK",
			want: "        " + pr.Number.Sprint("int") + " actor_id" + pr.Key.Sprint(" PK,FK"),
		},
		"relationship_label": {
			in: `    actor ||--o{ film_actor : "fk"`,
			want: "    " + pr.Header.Sprint("actor") + " " + pr.Punc.Sprint("||--o{") +
				" " + pr.Header.Sprint("film_actor") + " : " + pr.String.Sprint(`"fk"`),
		},
		// sq always emits a label, but #691's grammar (and this colorizer)
		// handles the label-less form too.
		"relationship_no_label": {
			in: "    actor ||--o{ film_actor",
			want: "    " + pr.Header.Sprint("actor") + " " + pr.Punc.Sprint("||--o{") +
				" " + pr.Header.Sprint("film_actor"),
		},
		// sq never emits comments, but the grammar colors them faint.
		"comment": {
			in:   "    %% a note",
			want: "    " + pr.Faint.Sprint("%% a note"),
		},
		// A line matching no token shape is emitted unchanged rather than
		// corrupted.
		"unrecognized_passthrough": {
			in:   "    one two three four five",
			want: "    one two three four five",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, colorizeLine(tc.in, pr))
		})
	}
}
