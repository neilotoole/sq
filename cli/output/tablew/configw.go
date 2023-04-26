package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/cli/output"
)

var _ output.ConfigWriter = (*configWriter)(nil)

// configWriter implements output.ConfigWriter.
type configWriter struct {
	tbl *table
}

// NewConfigWriter returns a new output.ConfigWriter.
func NewConfigWriter(out io.Writer, pr *output.Printing) output.ConfigWriter {
	tbl := &table{out: out, pr: pr, header: pr.ShowHeader}
	tbl.reset()
	return &configWriter{tbl: tbl}
}

// Location implements output.ConfigWriter.
func (w *configWriter) Location(path, origin string) error {
	fmt.Fprintln(w.tbl.out, path)
	if w.tbl.pr.Verbose && origin != "" {
		w.tbl.pr.Faint.Fprint(w.tbl.out, "Origin: ")
		w.tbl.pr.String.Fprintln(w.tbl.out, origin)
	}

	return nil
}

// Options implements output.ConfigWriter.
func (w *configWriter) Options(opts options.Options) error {
	if opts == nil {
		return nil
	}

	t, pr := w.tbl.tblImpl, w.tbl.pr
	if pr.ShowHeader {
		t.SetHeader([]string{"KEY", "VALUE"})
	}
	t.SetColTrans(0, pr.Key.SprintFunc())

	// FIXME: Verbose functionality should show defaults
	var rows [][]string

	keys := opts.Keys()
	for _, k := range keys {
		rows = append(rows, []string{k, fmt.Sprintf("%v", opts[k])})

		// FIXME: need to check type of Opt from registry, and SetCellTrans
		// appropriately
	}

	// rows = append(rows, []string{"output_format", string(opts.Format)})
	//
	// rows = append(rows, []string{"output_header", strconv.FormatBool(opts.Header)})
	// t.SetCellTrans(1, 1, pr.Bool.SprintFunc())
	//
	// rows = append(rows, []string{"ping_timeout", fmt.Sprintf("%v", opts.PingTimeout)})
	// t.SetCellTrans(2, 1, pr.Datetime.SprintFunc())
	//
	// rows = append(rows, []string{"shell_completion_timeout", fmt.Sprintf("%v", opts.ShellCompletionTimeout)})
	// t.SetCellTrans(3, 1, pr.Datetime.SprintFunc())

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
