package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.SourceWriter = (*sourceWriter)(nil)

type sourceWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// NewSourceWriter returns a source writer that outputs source
// details in text table format.
func NewSourceWriter(out io.Writer, fm *output.Formatting) output.SourceWriter {
	return &sourceWriter{out: out, fm: fm}
}

// SourceSet implements output.SourceWriter.
func (w *sourceWriter) SourceSet(ss *source.Set) error {
	if ss == nil {
		return nil
	}

	ss = ss.Clone()
	items := ss.Items()
	for i := range items {
		items[i].Location = items[i].RedactedLocation()
	}

	return writeJSON(w.out, w.fm, ss.Data())
}

// Source implements output.SourceWriter.
func (w *sourceWriter) Source(src *source.Source) error {
	if src == nil {
		return nil
	}

	src = src.Clone()
	src.Location = src.RedactedLocation()
	return writeJSON(w.out, w.fm, src)
}

// Removed implements output.SourceWriter.
func (w *sourceWriter) Removed(srcs ...*source.Source) error {
	if !w.fm.Verbose || len(srcs) == 0 {
		return nil
	}

	srcs2 := make([]*source.Source, len(srcs))
	for i := range srcs {
		srcs2[i] = srcs[i].Clone()
		srcs2[i].Location = srcs2[i].RedactedLocation()
	}
	return writeJSON(w.out, w.fm, srcs2)
}
