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
	"the mermaid-erd format supports only source and table schema diagrams")

// metadataWriter implements output.MetadataWriter for the "mermaid-erd"
// format, emitting bare Mermaid.js erDiagram source.
type metadataWriter struct {
	out io.Writer
}

// NewMetadataWriter returns a new output.MetadataWriter that outputs bare
// Mermaid.js erDiagram source. The *output.Printing arg is accepted for
// call-site consistency with the other metadata writers but is unused: the
// diagram source is plain text with no color, redaction, or provenance.
func NewMetadataWriter(out io.Writer, _ *output.Printing) output.MetadataWriter {
	return &metadataWriter{out: out}
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

	_, err := io.WriteString(w.out, mermaid.SourceDiagram(tables))
	return err
}

// TableMetadata implements output.MetadataWriter, writing a focused
// single-table ERD.
func (w *metadataWriter) TableMetadata(md *metadata.Table) error {
	_, err := io.WriteString(w.out, mermaid.TableDiagram(md, nil))
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
