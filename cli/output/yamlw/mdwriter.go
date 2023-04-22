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
	return writeYAML(w.yp, w.out, md)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(md *source.TableMetadata) error {
	return writeYAML(w.yp, w.out, md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata) error {
	md2 := *md // Shallow copy is fine
	md2.Location = source.RedactLocation(md2.Location)

	return writeYAML(w.yp, w.out, &md2)
}
