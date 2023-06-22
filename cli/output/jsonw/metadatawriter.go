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
	pr  *output.Printing
}

// NewMetadataWriter returns a new output.MetadataWriter instance
// that outputs metadata in JSON.
func NewMetadataWriter(out io.Writer, pr *output.Printing) output.MetadataWriter {
	return &mdWriter{out: out, pr: pr}
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(md []driver.Metadata) error {
	return writeJSON(w.out, w.pr, md)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(md *source.TableMetadata) error {
	return writeJSON(w.out, w.pr, md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata, showSchema bool) error {
	md2 := *md // Shallow copy is fine
	md2.Location = source.RedactLocation(md2.Location)

	if showSchema {
		return writeJSON(w.out, w.pr, &md2)
	}

	// Don't render "tables", "table_count", and "view_count"
	type mdNoSchema struct {
		source.Metadata
		Tables     *[]*source.TableMetadata `json:"tables,omitempty"`
		TableCount *int64                   `json:"table_count,omitempty"`
		ViewCount  *int64                   `json:"view_count,omitempty"`
	}

	return writeJSON(w.out, w.pr, &mdNoSchema{Metadata: md2})
}

// DBProperties implements output.MetadataWriter.
func (w *mdWriter) DBProperties(props map[string]any) error {
	if len(props) == 0 {
		return nil
	}
	return writeJSON(w.out, w.pr, props)
}
