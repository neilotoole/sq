package tablew

import (
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

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
	if !w.verbose {
		// Print the short version
		var rows [][]string

		for i, src := range ss.Items() {
			row := []string{
				src.Handle,
				string(src.Type),
				source.ShortLocation(src.Location),
			}

			if ss.Active() != nil && ss.Active().Handle == src.Handle {
				row[0] = w.tbl.fm.Handle.Sprintf(row[0]) + "*" // add the star to indicate active src

				w.tbl.tblImpl.SetCellTrans(i, 0, w.tbl.fm.Bold.SprintFunc())
				w.tbl.tblImpl.SetCellTrans(i, 1, w.tbl.fm.Bold.SprintFunc())
				w.tbl.tblImpl.SetCellTrans(i, 2, w.tbl.fm.Bold.SprintFunc())
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
	for i, src := range ss.Items() {
		row := []string{
			src.Handle,
			string(src.Type),
			src.RedactedLocation(),
			renderSrcOptions(src)}

		if ss.Active() != nil && ss.Active().Handle == src.Handle {
			row[0] = w.tbl.fm.Handle.Sprintf(row[0]) + "*" // add the star to indicate active src

			w.tbl.tblImpl.SetCellTrans(i, 0, w.tbl.fm.Bold.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 1, w.tbl.fm.Bold.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 2, w.tbl.fm.Bold.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 3, w.tbl.fm.Bold.SprintFunc())
			w.tbl.tblImpl.SetCellTrans(i, 4, w.tbl.fm.Bold.SprintFunc())
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
		renderSrcOptions(src)}
	rows = append(rows, row)

	w.tbl.tblImpl.SetColTrans(0, w.tbl.fm.Number.SprintFunc())
	w.tbl.tblImpl.SetHeaderDisable(true)
	w.tbl.appendRowsAndRenderAll(rows)
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
