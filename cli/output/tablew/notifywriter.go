package tablew

import (
	"io"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/notify"
)

type notifyWriter struct {
	tbl *table
}

// NewNotifyWriter implements output.NotificationWriter.
func NewNotifyWriter(out io.Writer, fm *output.Formatting) output.NotificationWriter {
	tbl := &table{out: out, header: fm.ShowHeader, fm: fm }
	w := &notifyWriter{tbl: tbl}
	w.tbl.reset()
	return w
}

// NotifyDestinations implements output.NotificationWriter.
func (w *notifyWriter) NotifyDestinations(dests []notify.Destination) error {
	w.tbl.tblImpl.SetHeader([]string{"NOTIFIER", "TYPE", "TARGET"})
	var rows [][]string

	for _, dest := range dests {
		row := []string{dest.Label, string(dest.Type), dest.Target}
		rows = append(rows, row)
	}

	w.tbl.appendRowsAndRenderAll(rows)
	return nil
}
