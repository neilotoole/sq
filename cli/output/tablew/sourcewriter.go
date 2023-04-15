package tablew

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.SourceWriter = (*sourceWriter)(nil)

type sourceWriter struct {
	tbl *table
}

// NewSourceWriter returns a source writer that outputs source
// details in text table format.
func NewSourceWriter(out io.Writer, fm *output.Formatting) output.SourceWriter {
	tbl := &table{out: out, fm: fm, header: fm.ShowHeader}
	w := &sourceWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// SourceSet implements output.SourceWriter.
func (w *sourceWriter) SourceSet(ss *source.Set) error {
	fm := w.tbl.fm
	group := ss.ActiveGroup()
	items, err := ss.SourcesInGroup(group)
	if err != nil {
		return err
	}

	if !fm.Verbose {
		// Print the short version
		var rows [][]string

		for _, src := range items {
			row := []string{
				src.Handle,
				string(src.Type),
				source.ShortLocation(src.Location),
			}

			if ss.Active() != nil && ss.Active().Handle == src.Handle {
				row[0] = fm.Active.Sprintf(row[0])
			}

			rows = append(rows, row)
		}

		w.tbl.tblImpl.SetHeaderDisable(true)
		w.tbl.tblImpl.SetColTrans(0, fm.Handle.SprintFunc())
		w.tbl.appendRowsAndRenderAll(rows)
		return nil
	}

	// Else print verbose

	// "HANDLE", "DRIVER", "LOCATION", "ACTIVE", "OPTIONS"
	var rows [][]string
	for _, src := range items {
		row := []string{
			src.Handle,
			string(src.Type),
			src.RedactedLocation(),
			"",
			renderSrcOptions(src),
		}

		if ss.Active() != nil && ss.Active().Handle == src.Handle {
			row[0] = fm.Active.Sprintf(row[0])
			row[3] = fm.Bool.Sprintf("active")
		}

		rows = append(rows, row)
	}

	w.tbl.tblImpl.SetHeaderDisable(!fm.ShowHeader)
	w.tbl.tblImpl.SetColTrans(0, fm.Handle.SprintFunc())
	w.tbl.tblImpl.SetHeader([]string{"HANDLE", "DRIVER", "LOCATION", "ACTIVE", "OPTIONS"})
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Source implements output.SourceWriter.
func (w *sourceWriter) Source(ss *source.Set, src *source.Source) error {
	if src == nil {
		return nil
	}

	var isActiveSrc bool
	if ss != nil && ss.Active() == src {
		isActiveSrc = true
	}

	if !w.tbl.fm.Verbose {
		var rows [][]string
		row := []string{
			src.Handle,
			string(src.Type),
			source.ShortLocation(src.Location),
		}
		rows = append(rows, row)

		if isActiveSrc {
			w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Active.SprintFunc())
		} else {
			w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
		}

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

	if isActiveSrc {
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Active.SprintFunc())
	} else {
		w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Handle.SprintFunc())
	}

	w.tbl.tblImpl.SetHeaderDisable(true)
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Removed implements output.SourceWriter.
func (w *sourceWriter) Removed(srcs ...*source.Source) error {
	if !w.tbl.fm.Verbose || len(srcs) == 0 {
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
func (w *sourceWriter) Group(group *source.Group) error {
	if group == nil {
		return nil
	}
	fm := w.tbl.fm

	if !fm.Verbose {
		if group.Active {
			_, err := fm.Active.Fprintln(w.tbl.out, group)
			return err
		}
		_, err := fm.Handle.Fprintln(w.tbl.out, group)
		return err
	}

	// fm.Verbose is true
	return w.renderGroups([]*source.Group{group})
}

// SetActiveGroup implements output.SourceWriter.
func (w *sourceWriter) SetActiveGroup(group *source.Group) error {
	if !w.tbl.fm.Verbose {
		// Only print the group if --verbose
		return nil
	}

	_, err := w.tbl.fm.Active.Fprintln(w.tbl.out, group)
	return err
}

func (w *sourceWriter) renderGroups(groups []*source.Group) error {
	fm := w.tbl.fm

	if !fm.Verbose {
		for _, group := range groups {
			if group.Active {
				fm.Active.Fprintln(w.tbl.out, group.Name)
				continue
			}

			fm.Handle.Fprintln(w.tbl.out, group.Name)
		}
		return nil
	}

	// Verbose output
	headers := []string{
		"GROUP",
		"SOURCES",
		"TOTAL",
		"SUBGROUPS",
		"TOTAL",
		"ACTIVE",
	}
	w.tbl.tblImpl.SetHeader(headers)

	var rows [][]string
	for _, g := range groups {
		directSrcCount, totalSrcCount, directGroupCount, totalGroupCount := g.Counts()
		row := []string{
			g.Name,
			strconv.Itoa(directSrcCount),
			strconv.Itoa(totalSrcCount),
			strconv.Itoa(directGroupCount),
			strconv.Itoa(totalGroupCount),
			strconv.FormatBool(g.Active),
		}

		if g.Active {
			row[0] = fm.Active.Sprintf(row[0])
			row[5] = fm.Bool.Sprintf("active")
		} else {
			// Don't render value for active==false. It's just noise.
			row[5] = ""
		}

		rowEmptyZeroes(fm, row)
		rows = append(rows, row)
	}

	w.tbl.tblImpl.SetColTrans(0, fm.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, fm.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, fm.Bool.SprintFunc())

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Groups implements output.SourceWriter.
func (w *sourceWriter) Groups(tree *source.Group) error {
	groups := tree.AllGroups()
	return w.renderGroups(groups)
}

// rowEmptyZeroes sets "0" to empty string. This seems to
// help with visual clutter.
func rowEmptyZeroes(_ *output.Formatting, row []string) {
	for i := range row {
		if row[i] == "0" {
			row[i] = ""
		}
	}
}

// rowEmptyZeroes prints "0" via fm.Faint. This seems to
// help with visual clutter.
func rowFaintZeroes(fm *output.Formatting, row []string) { //nolint:unused
	for i := range row {
		if row[i] == "0" {
			row[i] = fm.Faint.Sprintf(row[i])
		}
	}
}
