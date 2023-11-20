package yamlw

import (
	"io"

	"github.com/samber/lo"

	yamlp "github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ output.MetadataWriter = (*mdWriter)(nil)

// mdWriter implements output.MetadataWriter for YAML.
type mdWriter struct {
	out io.Writer
	pr  *output.Printing
	yp  yamlp.Printer
}

// NewMetadataWriter returns a new output.MetadataWriter instance
// that outputs metadata in JSON.
func NewMetadataWriter(out io.Writer, pr *output.Printing) output.MetadataWriter {
	return &mdWriter{out: out, pr: pr, yp: newPrinter(pr)}
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(md []driver.Metadata) error {
	return writeYAML(w.out, w.yp, md)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(md *metadata.Table) error {
	return writeYAML(w.out, w.yp, md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *metadata.Source, showSchema bool) error {
	md2 := *md // Shallow copy is fine
	md2.Location = source.RedactLocation(md2.Location)

	if showSchema {
		return writeYAML(w.out, w.yp, &md2)
	}

	// Don't render "tables", "table_count", and "view_count"
	type mdNoSchema struct {
		metadata.Source `yaml:",omitempty,inline"`
		Tables          *[]*metadata.Table `yaml:"tables,omitempty"`
		TableCount      *int64             `yaml:"table_count,omitempty"`
		ViewCount       *int64             `yaml:"view_count,omitempty"`
	}

	return writeYAML(w.out, w.yp, &mdNoSchema{Source: md2})
}

// DBProperties implements output.MetadataWriter.
func (w *mdWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}

	return writeYAML(w.out, w.yp, props)
}

// Catalogs implements output.MetadataWriter.
func (w *mdWriter) Catalogs(currentCatalog string, catalogs []string) error {
	if len(catalogs) == 0 {
		return nil
	}

	type cat struct {
		Name   string `yaml:"catalog"`
		Active *bool  `yaml:"active,omitempty"`
	}

	cats := make([]cat, len(catalogs))
	for i, c := range catalogs {
		cats[i] = cat{Name: c}
		if c == currentCatalog {
			cats[i].Active = lo.ToPtr(true)
		}
	}
	return writeYAML(w.out, w.yp, cats)
}

// Schemata implements output.MetadataWriter.
func (w *mdWriter) Schemata(currentSchema string, schemas []*metadata.Schema) error {
	if len(schemas) == 0 {
		return nil
	}

	// We wrap each schema in a struct that has an "active" field,
	// because we need to show the current schema in the output.
	type wrapper struct {
		metadata.Schema `yaml:",omitempty,inline"`
		Active          *bool `yaml:"active,omitempty"`
	}

	a := make([]*wrapper, len(schemas))
	for i, s := range schemas {
		a[i] = &wrapper{Schema: *s}
		if s.Name == currentSchema {
			a[i].Active = lo.ToPtr(true)
		}
	}

	return writeYAML(w.out, w.yp, a)
}
