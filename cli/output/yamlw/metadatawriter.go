package yamlw

import (
	"io"

	yamlp "github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

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
func (w *mdWriter) TableMetadata(md *source.TableMetadata) error {
	return writeYAML(w.out, w.yp, md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata, showSchema bool) error {
	md2 := *md // Shallow copy is fine
	md2.Location = source.RedactLocation(md2.Location)

	if showSchema {
		return writeYAML(w.out, w.yp, &md2)
	}

	// Don't render "tables", "table_count", and "view_count"
	type mdNoSchema struct {
		source.Metadata `yaml:",omitempty,inline"`
		Tables          *[]*source.TableMetadata `yaml:"tables,omitempty"`
		TableCount      *int64                   `yaml:"table_count,omitempty"`
		ViewCount       *int64                   `yaml:"view_count,omitempty"`
	}

	return writeYAML(w.out, w.yp, &mdNoSchema{Metadata: md2})
}

// DBProperties implements output.MetadataWriter.
func (w *mdWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}

	return writeYAML(w.out, w.yp, props)
}
