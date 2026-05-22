package markdownw

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// writeSourceERD writes the whole-source entity-relationship diagram
// section (Markdown heading + fenced mermaid block).
func (w *metadataWriter) writeSourceERD(buf *bytes.Buffer, tables []*metadata.Table) {
	writeMermaidSection(buf, mermaid.SourceDiagram(tables), 2)
}

// writeTableERD writes a focused entity-relationship diagram section for a
// single table at the given Markdown heading level. cardIndex (may be nil)
// supplies neighbor tables for cardinality inference.
func (w *metadataWriter) writeTableERD(
	buf *bytes.Buffer, tbl *metadata.Table, headingLevel int, cardIndex map[string]*metadata.Table,
) {
	writeMermaidSection(buf, mermaid.TableDiagram(tbl, cardIndex), headingLevel)
}

// writeMermaidSection wraps bare Mermaid source in the Markdown
// "Entity Relationship Diagram" heading and fenced code block. It writes
// nothing when src is empty.
func writeMermaidSection(buf *bytes.Buffer, src string, headingLevel int) {
	if src == "" {
		return
	}
	fmt.Fprintf(buf, "\n%s Entity Relationship Diagram\n\n```mermaid\n%s```\n",
		strings.Repeat("#", headingLevel), src)
}
