// Package mermaidw implements output.MetadataWriter for the "mermaid-erd"
// format: sq inspect's bare Mermaid.js erDiagram source, with none of the
// surrounding schema document that the markdownw and htmlw writers produce.
//
// It supports only source and table schema inspection (SourceMetadata and
// TableMetadata); the other metadata operations have no diagram
// representation and return errUnsupported.
package mermaidw

import (
	"cmp"
	"io"
	"slices"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/internal/mermaid"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ output.MetadataWriter = (*metadataWriter)(nil)

// errUnsupported is returned by the metadata operations that have no Mermaid
// ERD representation.
var errUnsupported = errz.New(
	"the mermaid-erd format supports only source and table schema diagrams",
)

// errNothingToRender is returned when there's no diagram to draw, i.e. no
// tables with columns and no foreign keys. Unlike markdownw/htmlw, where the
// ERD is one section of a larger document, here the diagram is the entire
// output, so an empty render is an error rather than silent empty output.
var errNothingToRender = errz.New(
	"the mermaid-erd format has nothing to render: no columns or foreign keys found",
)

// metadataWriter implements output.MetadataWriter for the "mermaid-erd"
// format, emitting bare Mermaid.js erDiagram source.
type metadataWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewMetadataWriter returns a new output.MetadataWriter that outputs bare
// Mermaid.js erDiagram source. When pr enables color (a TTY sink), the diagram
// is syntax-colored; in monochrome mode (files, pipes, --no-color, NO_COLOR,
// --monochrome) the output is plain, byte-identical to the bare source.
func NewMetadataWriter(out io.Writer, pr *output.Printing) output.MetadataWriter {
	return &metadataWriter{out: out, pr: pr}
}

// SourceMetadata implements output.MetadataWriter. It writes the whole-source
// ERD. Overview mode (showSchema=false) carries no table schema, so there's
// nothing to diagram and it returns errUnsupported.
func (w *metadataWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	if !showSchema {
		return errUnsupported
	}

	// Render with a stable table ordering (tables before views, then by
	// name), matching the markdownw/htmlw ERDs.
	tables := append([]*metadata.Table(nil), md.Tables...)
	slices.SortFunc(tables, compareTables)

	return w.writeDiagram(mermaid.SourceDiagram(tables))
}

// TableMetadata implements output.MetadataWriter, writing a focused
// single-table ERD.
func (w *metadataWriter) TableMetadata(md *metadata.Table) error {
	return w.writeDiagram(mermaid.TableDiagram(md, nil))
}

// writeDiagram writes the rendered diagram source to w.out, returning
// errNothingToRender when src is empty (the mermaid package returns "" when
// there's nothing to draw). On a TTY sink the source is syntax-colored;
// otherwise it's written plain.
func (w *metadataWriter) writeDiagram(src string) error {
	if src == "" {
		return errNothingToRender
	}
	_, err := io.WriteString(w.out, colorize(src, w.pr))
	return err
}

// DBProperties implements output.MetadataWriter. DB properties have no ERD
// representation.
func (w *metadataWriter) DBProperties(map[string]any) error {
	return errUnsupported
}

// DriverMetadata implements output.MetadataWriter. The driver list has no ERD
// representation.
func (w *metadataWriter) DriverMetadata([]driver.Metadata) error {
	return errUnsupported
}

// Catalogs implements output.MetadataWriter. A catalog list has no ERD
// representation.
func (w *metadataWriter) Catalogs(string, []string) error {
	return errUnsupported
}

// Schemata implements output.MetadataWriter. A schema list has no ERD
// representation.
func (w *metadataWriter) Schemata(string, []*metadata.Schema) error {
	return errUnsupported
}

// compareTables orders tables before views, then by name, so the emitted
// diagram is deterministic.
func compareTables(a, b *metadata.Table) int {
	if a.TableType == b.TableType {
		return cmp.Compare(a.Name, b.Name)
	}
	return cmp.Compare(a.TableType, b.TableType)
}
