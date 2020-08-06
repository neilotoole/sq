package tablew

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/notify"
)

type NotifyWriter struct {
	tbl *table
}

func NewNotifyWriter(out io.Writer, fm *output.Formatting, header bool) *NotifyWriter {
	tbl := &table{out: out, header: header, fm: fm}
	w := &NotifyWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

func (w *NotifyWriter) NotifyDestinations(dests []notify.Destination) error {
	w.tbl.tblImpl.SetHeader([]string{"NOTIFIER", "TYPE", "TARGET"})
	var rows [][]string

	for _, dest := range dests {
		row := []string{dest.Label, string(dest.Type), dest.Target}
		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
