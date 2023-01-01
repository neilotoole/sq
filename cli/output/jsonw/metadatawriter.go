package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// mdWriter implements output.MetadataWriter for JSON.
type mdWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// NewMetadataWriter returns a new output.MetadataWriter instance
// that outputs metadata in JSON.
func NewMetadataWriter(out io.Writer, fm *output.Formatting) output.MetadataWriter {
	return &mdWriter{out: out, fm: fm}
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(md []driver.Metadata) error {
	return writeJSON(w.out, w.fm, md)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(md *source.TableMetadata) error {
	return writeJSON(w.out, w.fm, md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata) error {
	md2 := *md // Shallow copy is fine
	md2.Location = source.RedactLocation(md2.Location)

	return writeJSON(w.out, w.fm, &md2)
}
