package tablew

import (
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.SourceWriter = (*sourceWriter)(nil)

type sourceWriter struct {
	tbl     *table
	verbose bool
}

// NewSourceWriter returns a source writer that outputs source
// details in text table format.
func NewSourceWriter(out io.Writer, fm *output.Formatting) output.SourceWriter {
	tbl := &table{out: out, fm: fm, header: fm.ShowHeader}
	w := &sourceWriter{tbl: tbl, verbose: fm.Verbose}
	w.tbl.reset()
	return w
}

// SourceSet implements output.SourceWriter.
func (w *sourceWriter) SourceSet(ss *source.Set) error {
	group := ss.ActiveGroup()
	items, err := ss.SourcesInGroup(group)
	if err != nil {
		return err
	}

	if !w.verbose {
		// Print the short version
		var rows [][]string

		for i, src := range items {
			row := []string{
				src.Handle,
				string(src.Type),
				source.ShortLocation(src.Location),
			}

			if ss.Active() != nil && ss.Active().Handle == src.Handle {
				row[0] = w.tbl.fm.Active.Sprintf(row[0])

				w.tbl.tblImpl.SetCellTrans(i, 0, w.tbl.fm.Active.SprintFunc())
				w.tbl.tblImpl.SetCellTrans(i, 1, w.tbl.fm.Active.SprintFunc())
				w.tbl.tblImpl.SetCellTrans(i, 2, w.tbl.fm.Active.SprintFunc())
			}

			rows = append(rows, row)
		}

		w.tbl.tblImpl.SetHeaderDisable(true)
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
		w.tbl.appendRowsAndRenderAll(rows)
		return nil
	}

	// Else print verbose

	// "HANDLE", "DRIVER", "LOCATION", "OPTIONS"
	var rows [][]string
	for i, src := range items {
		row := []string{
			src.Handle,
			string(src.Type),
			src.RedactedLocation(),
			renderSrcOptions(src),
		}

		if ss.Active() != nil && ss.Active().Handle == src.Handle {
			row[0] = w.tbl.fm.Active.Sprintf(row[0])

			w.tbl.tblImpl.SetCellTrans(i, 0, w.tbl.fm.Active.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 1, w.tbl.fm.Active.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 2, w.tbl.fm.Active.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 3, w.tbl.fm.Active.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 4, w.tbl.fm.Active.SprintFunc())
		}

		rows = append(rows, row)
	}

	w.tbl.tblImpl.SetHeaderDisable(!w.tbl.fm.ShowHeader)
	w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
	w.tbl.tblImpl.SetHeader([]string{"HANDLE", "DRIVER", "LOCATION", "OPTIONS"})
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Source implements output.SourceWriter.
func (w *sourceWriter) Source(src *source.Source) error {
	if !w.verbose {
		var rows [][]string
		row := []string{
			src.Handle,
			string(src.Type),
			source.ShortLocation(src.Location),
		}
		rows = append(rows, row)
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Number.SprintFunc())
		w.tbl.tblImpl.SetHeaderDisable(true)
		w.tbl.appendRowsAndRenderAll(rows)
		return nil
	}

	var rows [][]string
	row := []string{
		src.Handle,
		string(src.Type),
		src.RedactedLocation(),
		renderSrcOptions(src),
	}
	rows = append(rows, row)

	w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetHeaderDisable(true)
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Removed implements output.SourceWriter.
func (w *sourceWriter) Removed(srcs ...*source.Source) error {
	if !w.verbose || len(srcs) == 0 {
		return nil
	}

	w.tbl.fm.Faint.Fprint(w.tbl.out, "Removed ")
	w.tbl.fm.Number.Fprint(w.tbl.out, len(srcs))
	w.tbl.fm.Faint.Fprintln(w.tbl.out, " sources")

	for _, src := range srcs {
		w.tbl.fm.Handle.Fprintln(w.tbl.out, src.Handle)
	}
	return nil
}

func renderSrcOptions(src *source.Source) string {
	if src == nil || src.Options == nil || len(src.Options) == 0 {
		return ""
	}

	opts := make([]string, 0, len(src.Options))

	for key, vals := range src.Options {
		if key == "" {
			continue
		}
		v := strings.Join(vals, ",")
		// TODO: add color here to distinguish the keys/values
		opts = append(opts, fmt.Sprintf("%s=%s", key, v))
	}
	return strings.Join(opts, " ")
}

// Group implements output.SourceWriter.
func (w *sourceWriter) Group(group string) error {
	if group == "" {
		return nil
	}

	_, err := w.tbl.fm.Handle.Fprintln(w.tbl.out, group)
	return err
}

// SetActiveGroup implements output.SourceWriter.
func (w *sourceWriter) SetActiveGroup(group string) error {
	if !w.tbl.fm.Verbose {
		// Only print the group if --verbose
		return nil
	}

	_, err := w.tbl.fm.Active.Fprintln(w.tbl.out, group)
	return err
}

// Groups implements output.SourceWriter.
func (w *sourceWriter) Groups(activeGroup string, groups []string) error {
	for _, group := range groups {
		if group == activeGroup {
			w.tbl.fm.Active.Fprintln(w.tbl.out, group)
			continue
		}

		w.tbl.fm.Handle.Fprintln(w.tbl.out, group)
	}
	return nil
}
