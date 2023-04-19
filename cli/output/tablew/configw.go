package tablew

import (
	"fmt"
	"io"
	"strconv"

	"github.com/neilotoole/sq/cli/config"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	tbl *table
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, fm *output.Formatting) output.ConfigWriter {
	tbl := &table{out: out, fm: fm, header: fm.ShowHeader}
	tbl.reset()
	return &configWriter{tbl: tbl}
}

// Location implements output.ConfigWriter.
func (w *configWriter) Location(path, origin string) error {
	fmt.Fprintln(w.tbl.out, path)
	if w.tbl.fm.Verbose && origin != "" {
		w.tbl.fm.Faint.Fprint(w.tbl.out, "Origin: ")
		w.tbl.fm.String.Fprintln(w.tbl.out, origin)
	}

	return nil
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(opts *config.Options) error {
	if opts == nil {
		return nil
	}

	t, fm := w.tbl.tblImpl, w.tbl.fm
	if fm.ShowHeader {
		t.SetHeader([]string{"KEY", "VALUE"})
	}
	t.SetColTrans(0, fm.Key.SprintFunc())

	var rows [][]string
	rows = append(rows, []string{"output_format", string(opts.Format)})

	rows = append(rows, []string{"output_header", strconv.FormatBool(opts.Header)})
	t.SetCellTrans(1, 1, fm.Bool.SprintFunc())

	rows = append(rows, []string{"ping_timeout", fmt.Sprintf("%v", opts.PingTimeout)})
	t.SetCellTrans(2, 1, fm.Datetime.SprintFunc())

	rows = append(rows, []string{"shell_completion_timeout", fmt.Sprintf("%v", opts.ShellCompletionTimeout)})
	t.SetCellTrans(3, 1, fm.Datetime.SprintFunc())

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
