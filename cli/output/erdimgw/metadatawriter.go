// Package erdimgw implements output.MetadataWriter for the "png-erd" and
// "svg-erd" formats: sq inspect's schema entity-relationship diagram
// rendered to a PNG or SVG image file. The diagram is generated as Graphviz
// DOT (see cli/output/internal/erddot) and rasterized by go-graphviz, which
// embeds Graphviz as WebAssembly — so image export needs no external tool,
// browser, or network.
//
// Like mermaidw, it supports only source and table schema inspection
// (SourceMetadata and TableMetadata); the other metadata operations have no
// ERD representation and return errUnsupported.
package erdimgw

import (
	"cmp"
	"context"
	"io"
	"slices"
	"sync"

	graphviz "github.com/goccy/go-graphviz"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/internal/erddot"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// renderMu serializes go-graphviz rendering. go-graphviz runs Graphviz via a
// process-global WebAssembly runtime that isn't safe for concurrent use, so
// concurrent renders can corrupt its shared state. A single sq inspect renders
// exactly one diagram, so in practice this lock is uncontended; it's here to
// keep the writer safe if ever driven concurrently (e.g. by parallel tests).
var renderMu sync.Mutex

var _ output.MetadataWriter = (*metadataWriter)(nil)

// errUnsupported is returned by the metadata operations that have no ERD
// representation.
var errUnsupported = errz.New(
	"the png-erd and svg-erd formats support only source and table schema diagrams",
)

// errNothingToRender is returned when there's no diagram to draw, i.e. no
// tables with columns and no foreign keys. As with mermaidw, the diagram is
// the entire output, so an empty render is an error rather than an empty
// (and invalid) image file.
var errNothingToRender = errz.New(
	"the png-erd/svg-erd format has nothing to render: no columns or foreign keys found",
)

// metadataWriter implements output.MetadataWriter for the "png-erd" and
// "svg-erd" formats, rendering the schema ERD to an image via go-graphviz.
type metadataWriter struct {
	out    io.Writer
	ctx    context.Context
	format graphviz.Format
}

// NewPNGMetadataWriter returns an output.MetadataWriter that renders the
// schema ERD to a PNG image. The *output.Printing arg is accepted for
// call-site consistency with the other metadata writers but is unused: the
// rendered image carries no sq-side color, redaction, or provenance.
func NewPNGMetadataWriter(out io.Writer, _ *output.Printing) output.MetadataWriter {
	return &metadataWriter{out: out, ctx: context.Background(), format: graphviz.PNG}
}

// NewSVGMetadataWriter returns an output.MetadataWriter that renders the
// schema ERD to an SVG image. See NewPNGMetadataWriter regarding the unused
// *output.Printing arg.
func NewSVGMetadataWriter(out io.Writer, _ *output.Printing) output.MetadataWriter {
	return &metadataWriter{out: out, ctx: context.Background(), format: graphviz.SVG}
}

// SourceMetadata implements output.MetadataWriter. It renders the
// whole-source ERD. Overview mode (showSchema=false) carries no table
// schema, so there's nothing to diagram and it returns errUnsupported.
func (w *metadataWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	if !showSchema {
		return errUnsupported
	}

	// Render with a stable table ordering (tables before views, then by
	// name), matching the mermaidw/markdownw/htmlw ERDs.
	tables := append([]*metadata.Table(nil), md.Tables...)
	slices.SortFunc(tables, compareTables)

	return w.render(erddot.SourceDiagram(tables))
}

// TableMetadata implements output.MetadataWriter, rendering a focused
// single-table ERD.
func (w *metadataWriter) TableMetadata(md *metadata.Table) error {
	return w.render(erddot.TableDiagram(md, nil))
}

// render rasterizes the DOT source to w.format and writes the image bytes to
// w.out. It returns errNothingToRender when dot is empty (erddot returns ""
// when there's nothing to draw).
func (w *metadataWriter) render(dot string) error {
	if dot == "" {
		return errNothingToRender
	}

	renderMu.Lock()
	defer renderMu.Unlock()

	g, err := graphviz.New(w.ctx)
	if err != nil {
		return errz.Err(err)
	}
	// Close only tears down the WASM runtime/graph after Render has already
	// written every byte to w.out; a teardown error can't corrupt or truncate
	// the emitted image, and the process is short-lived, so it's safe to drop.
	defer func() { _ = g.Close() }()
	g.SetLayout(graphviz.DOT)

	graph, err := graphviz.ParseBytes([]byte(dot))
	if err != nil {
		return errz.Wrap(err, "render ERD: parse DOT")
	}
	defer func() { _ = graph.Close() }()

	// Render writes the whole image in a single Write; its error (including any
	// write failure to w.out) is surfaced here.
	return errz.Err(g.Render(w.ctx, graph, w.format, w.out))
}

// DBProperties implements output.MetadataWriter. DB properties have no ERD
// representation.
func (w *metadataWriter) DBProperties(map[string]any) error {
	return errUnsupported
}

// DriverMetadata implements output.MetadataWriter. The driver list has no
// ERD representation.
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
