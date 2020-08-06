package tablew

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

// NewPingWriter returns a new instance. It is not safe for
// concurrent use.
func NewPingWriter(out io.Writer, fm *output.Formatting) *PingWriter {
	return &PingWriter{out: out, fm: fm}
}

// PingWriter implements output.PingWriter.
type PingWriter struct {
	out io.Writer
	fm  *output.Formatting

	// handleLenMax is the maximum width of any of
	// the sources' handles.
	handleWidthMax int
}

func (w *PingWriter) Open(srcs []*source.Source) {
	for _, src := range srcs {
		if len(src.Handle) > w.handleWidthMax {
			w.handleWidthMax = len(src.Handle)
		}
	}
}

func (w *PingWriter) Result(src *source.Source, d time.Duration, err error) {
	w.fm.Number.Fprintf(w.out, "%-"+strconv.Itoa(w.handleWidthMax)+"s", src.Handle)
	fmt.Fprintf(w.out, "%10s  ", d.Truncate(time.Millisecond).String())

	// The ping result is one of:
	// - success
	// - timeout
	// - some other error

	switch {
	case err == nil:
		w.fm.Success.Fprintf(w.out, "pong")

	case err == context.DeadlineExceeded:
		w.fm.Error.Fprintf(w.out, "fail")
		// Special rendering for timeout error
		fmt.Fprint(w.out, "  timeout exceeded")

	default: // err other than timeout err
		w.fm.Error.Fprintf(w.out, "fail")
		fmt.Fprintf(w.out, "  %s", err)
	}

	fmt.Fprintf(w.out, "\n")
}

func (w *PingWriter) Close() error {
	return nil
}
