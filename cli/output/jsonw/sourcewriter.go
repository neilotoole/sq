package jsonw

import (
	"io"

	"golang.org/x/exp/slices"

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

	// This is a bit hacky. Basically we want to JSON-print ss.Data().
	// But, we want to do it just for the active group.
	// So, our hack is that we clone the source set, and remove any
	// sources that are not in the active group.
	//
	// This whole function, including what it outputs, should be revisited.
	ss = ss.Clone()
	group := ss.ActiveGroup()

	// We store the active src handle
	activeHandle := ss.ActiveHandle()

	handles, err := ss.HandlesInGroup(group)
	if err != nil {
		return err
	}

	srcs := ss.Sources()

	for _, src := range srcs {
		if !slices.Contains(handles, src.Handle) {
			if err = ss.Remove(src.Handle); err != nil {
				// Should never happen
				return err
			}
		}
	}

	srcs = ss.Sources()
	for i := range srcs {
		srcs[i].Location = srcs[i].RedactedLocation()
	}

	// HACK: we set the activeHandle back, even though that
	// active source may have been removed (because it is not in
	// the active group). This whole thing is a mess.
	_, _ = ss.SetActive(activeHandle, true)

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

// ActiveGroup implements output.SourceWriter.
func (w *sourceWriter) ActiveGroup(group string) error {
	if group == "" {
		group = "/"
	}
	m := map[string]string{"group": group}
	return writeJSON(w.out, w.fm, m)
}

// SetActiveGroup implements output.SourceWriter.
func (w *sourceWriter) SetActiveGroup(group string) error {
	if group == "" {
		group = "/"
	}
	m := map[string]string{"group": group}
	return writeJSON(w.out, w.fm, m)
}

// Groups implements output.SourceWriter.
func (w *sourceWriter) Groups(activeGroup string, groups []string) error {
	_ = activeGroup
	return writeJSON(w.out, w.fm, groups)
}
