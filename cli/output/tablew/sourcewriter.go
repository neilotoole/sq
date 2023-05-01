package tablew

import (
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
func NewSourceWriter(out io.Writer, pr *output.Printing) output.SourceWriter {
	tbl := &table{out: out, pr: pr, header: pr.ShowHeader}
	w := &sourceWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// Collection implements output.SourceWriter.
func (w *sourceWriter) Collection(coll *source.Collection) error {
	pr := w.tbl.pr
	group := coll.ActiveGroup()
	items, err := coll.SourcesInGroup(group)
	if err != nil {
		return err
	}

	if !pr.Verbose {
		// Print the short version
		var rows [][]string

		for _, src := range items {
			row := []string{
				src.Handle,
				string(src.Type),
				source.ShortLocation(src.Location),
			}

			if coll.Active() != nil && coll.Active().Handle == src.Handle {
				row[0] = pr.Active.Sprintf(row[0])
			}

			rows = append(rows, row)
		}

		w.tbl.tblImpl.SetHeaderDisable(true)
		w.tbl.tblImpl.SetColTrans(0, pr.Handle.SprintFunc())
		w.tbl.tblImpl.SetColTrans(2, pr.Location.SprintFunc())
		w.tbl.appendRowsAndRenderAll(rows)
		return nil
	}

	// Else print verbose

	// "HANDLE", "ACTIVE", "DRIVER", "LOCATION", "OPTIONS"
	var rows [][]string
	for _, src := range items {
		row := []string{
			src.Handle,
			"",
			string(src.Type),
			src.RedactedLocation(),
			renderSrcOptions(pr, src),
		}

		if coll.Active() != nil && coll.Active().Handle == src.Handle {
			row[0] = pr.Active.Sprintf(row[0])
			row[1] = pr.Bool.Sprintf("active")
		}

		rows = append(rows, row)
	}

	w.tbl.tblImpl.SetHeaderDisable(!pr.ShowHeader)
	w.tbl.tblImpl.SetColTrans(0, pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, pr.Location.SprintFunc())
	w.tbl.tblImpl.SetHeader([]string{"HANDLE", "ACTIVE", "DRIVER", "LOCATION", "OPTIONS"})
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Source implements output.SourceWriter.
func (w *sourceWriter) Source(coll *source.Collection, src *source.Source) error {
	if src == nil {
		return nil
	}

	var isActiveSrc bool
	if coll != nil && coll.Active() == src {
		isActiveSrc = true
	}

	if !w.tbl.pr.Verbose {
		var rows [][]string
		row := []string{
			src.Handle,
			string(src.Type),
			source.ShortLocation(src.Location),
		}
		rows = append(rows, row)

		if isActiveSrc {
			w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Active.SprintFunc())
		} else {
			w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
		}

		w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Location.SprintFunc())
		w.tbl.tblImpl.SetHeaderDisable(true)
		w.tbl.appendRowsAndRenderAll(rows)
		return nil
	}

	var rows [][]string
	row := []string{
		src.Handle,
		string(src.Type),
		src.RedactedLocation(),
		renderSrcOptions(w.tbl.pr, src),
	}
	rows = append(rows, row)

	if isActiveSrc {
		w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Active.SprintFunc())
	} else {
		w.tbl.tblImpl.SetColTrans(0, w.tbl.pr.Handle.SprintFunc())
	}

	w.tbl.tblImpl.SetColTrans(2, w.tbl.pr.Location.SprintFunc())
	w.tbl.tblImpl.SetHeaderDisable(true)
	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}

// Removed implements output.SourceWriter.
func (w *sourceWriter) Removed(srcs ...*source.Source) error {
	if !w.tbl.pr.Verbose || len(srcs) == 0 {
		return nil
	}

	w.tbl.pr.Faint.Fprint(w.tbl.out, "Removed ")
	w.tbl.pr.Number.Fprint(w.tbl.out, len(srcs))
	w.tbl.pr.Faint.Fprintln(w.tbl.out, " sources")

	for _, src := range srcs {
		w.tbl.pr.Handle.Fprintln(w.tbl.out, src.Handle)
	}
	return nil
}

func renderSrcOptions(pr *output.Printing, src *source.Source) string {
	if src == nil || src.Options == nil || len(src.Options) == 0 {
		return ""
	}

	opts := make([]string, 0, len(src.Options))

	for key, val := range src.Options {
		if key == "" {
			continue
		}
		clr := getColorForVal(pr, val)
		s := pr.Faint.Sprint(pr.Key.Sprintf("%s", key)) +
			pr.Faint.Sprint("=") + pr.Faint.Sprint(clr.Sprintf("%v", val))
		opts = append(opts, s)
	}
	return strings.Join(opts, " ")
}

// Group implements output.SourceWriter.
func (w *sourceWriter) Group(group *source.Group) error {
	if group == nil {
		return nil
	}
	pr := w.tbl.pr

	if !pr.Verbose {
		if group.Active {
			_, err := pr.Active.Fprintln(w.tbl.out, group)
			return err
		}
		_, err := pr.Handle.Fprintln(w.tbl.out, group)
		return err
	}

	// pr.Verbose is true
	return w.renderGroups([]*source.Group{group})
}

// SetActiveGroup implements output.SourceWriter.
func (w *sourceWriter) SetActiveGroup(group *source.Group) error {
	if !w.tbl.pr.Verbose {
		// Only print the group if --verbose
		return nil
	}

	_, err := w.tbl.pr.Active.Fprintln(w.tbl.out, group)
	return err
}

func (w *sourceWriter) renderGroups(groups []*source.Group) error {
	pr := w.tbl.pr

	if !pr.Verbose {
		for _, group := range groups {
			if group.Active {
				pr.Active.Fprintln(w.tbl.out, group.Name)
				continue
			}

			pr.Handle.Fprintln(w.tbl.out, group.Name)
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
			row[0] = pr.Active.Sprintf(row[0])
			row[5] = pr.Bool.Sprintf("active")
		} else {
			// Don't render value for active==false. It's just noise.
			row[5] = ""
		}

		rowEmptyZeroes(pr, row)
		rows = append(rows, row)
	}

	w.tbl.tblImpl.SetColTrans(0, pr.Handle.SprintFunc())
	w.tbl.tblImpl.SetColTrans(1, pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(2, pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(3, pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(4, pr.Number.SprintFunc())
	w.tbl.tblImpl.SetColTrans(5, pr.Bool.SprintFunc())

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
func rowEmptyZeroes(_ *output.Printing, row []string) {
	for i := range row {
		if row[i] == "0" {
			row[i] = ""
		}
	}
}

// rowEmptyZeroes prints "0" via pr.Faint. This seems to
// help with visual clutter.
func rowFaintZeroes(pr *output.Printing, row []string) { //nolint:unused
	for i := range row {
		if row[i] == "0" {
			row[i] = pr.Faint.Sprintf(row[i])
		}
	}
}
