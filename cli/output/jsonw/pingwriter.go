package jsonw

import (
	"io"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw/internal"
	jcolorenc "github.com/neilotoole/sq/cli/output/jsonw/internal/jcolorenc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.PingWriter = (*pingWriter)(nil)

// NewPingWriter returns JSON impl of output.PingWriter.
func NewPingWriter(out io.Writer, fm *output.Formatting) output.PingWriter {
	return &pingWriter{out: out, fm: fm}
}

type pingWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// Open implements output.PingWriter.
func (p pingWriter) Open(srcs []*source.Source) error {
	return nil
}

// Result implements output.PingWriter.
func (p pingWriter) Result(src *source.Source, d time.Duration, err error) error {
	r := struct {
		*source.Source
		Pong     bool          `json:"pong"`
		Duration time.Duration `json:"duration"`
		Error    string        `json:"error,omitempty"`
	}{
		Source:   src,
		Pong:     err == nil,
		Duration: d,
	}

	if err != nil {
		r.Error = err.Error()
	}

	enc := jcolorenc.NewEncoder(p.out)
	enc.SetColors(internal.NewColors(p.fm))
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	return errz.Err(enc.Encode(r))
}

// Close implements output.PingWriter.
func (p pingWriter) Close() error {
	return nil
}