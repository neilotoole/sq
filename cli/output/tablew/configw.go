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
func (w *configWriter) Options(reg *options.Registry, o options.Options) error {
	if o == nil {
		return nil
	}

	if w.tbl.pr.Verbose {
		w.tbl.pr.ShowHeader = true
	} else {
		w.tbl.pr.ShowHeader = false
	}

	w.doPrintOptions(reg, o)
	return nil
}

// Options implements output.ConfigWriter.
func (w *configWriter) doPrintOptions(reg *options.Registry, o options.Options) {
	if o == nil {
		return
	}

	t, pr, verbose := w.tbl.tblImpl, w.tbl.pr, w.tbl.pr.Verbose

	if pr.ShowHeader {
		headers := []string{"KEY", "VALUE"}
		if verbose {
			headers = []string{"KEY", "VALUE", "DEFAULT"}
		}

		t.SetHeader(headers)
	}
	t.SetColTrans(0, pr.Key.SprintFunc())

	var rows [][]string

	keys := o.Keys()
	for _, k := range keys {
		opt := reg.Get(k)
		if opt == nil {
			// Shouldn't happen, but print anyway just in case
			row := []string{
				k,
				pr.Error.Sprintf("%v", o[k]),
			}
			if verbose {
				// Can't know default in this situation.
				row = append(row, "")
			}

			rows = append(rows, row)
			continue
		}

		clr := pr.String
		switch opt.(type) {
		case options.Bool:
			clr = pr.Bool
		case options.Int:
			clr = pr.Number
		default:
		}

		row := []string{
			k,
			clr.Sprintf("%v", o[k]),
		}

		if verbose {
			defaultVal := pr.Faint.Sprintf("%v", opt.GetAny(nil))
			//			defaultVal := pr.Faint.Sprintf(clr.Sprintf("%v", opt.GetAny(nil)))

			row = append(row, defaultVal)
		}

		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
}

// SetOption implements output.ConfigWriter.
func (w *configWriter) SetOption(_ *options.Registry, o options.Options, opt options.Opt) error {
	if !w.tbl.pr.Verbose {
		// No output unless verbose
		return nil
	}

	// It's verbose
	o = options.Effective(o, opt)
	w.tbl.pr.ShowHeader = false
	return w.Options(nil, o)
}
