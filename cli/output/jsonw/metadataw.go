package jsonw

import (
	"bytes"
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	jcolorenc "github.com/neilotoole/sq/cli/output/jsonw/internal/jcolorenc"
	"github.com/neilotoole/sq/libsq/core/errz"
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

func (w *mdWriter) write(v interface{}) error {
	buf := &bytes.Buffer{}

	enc := jcolorenc.NewEncoder(buf)
	enc.SetColors(internal.NewColors(w.fm))
	enc.SetEscapeHTML(false)
	if w.fm.Pretty {
		enc.SetIndent("", w.fm.Indent)
	}

	err := enc.Encode(v)
	if err != nil {
		return errz.Err(err)
	}

	_, err = fmt.Fprint(w.out, buf.String())
	if err != nil {
		return errz.Err(err)
	}

	return nil
}

// DriverMetadata implements output.MetadataWriter.
func (w *mdWriter) DriverMetadata(md []driver.Metadata) error {
	return w.write(md)
}

// TableMetadata implements output.MetadataWriter.
func (w *mdWriter) TableMetadata(md *source.TableMetadata) error {
	return w.write(md)
}

// SourceMetadata implements output.MetadataWriter.
func (w *mdWriter) SourceMetadata(md *source.Metadata) error {
	return w.write(md)
}
