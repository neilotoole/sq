package tablew

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

// NewPingWriter returns a new instance. It is not safe for
// concurrent use.
func NewPingWriter(out io.Writer, pr *output.Printing) *PingWriter {
	return &PingWriter{out: out, pr: pr}
}

// PingWriter implements output.PingWriter.
type PingWriter struct {
	out io.Writer
	pr  *output.Printing

	// handleLenMax is the maximum width of
	// the sources' handles.
	handleWidthMax int
}

// Open implements output.PingWriter.
func (w *PingWriter) Open(srcs []*source.Source) error {
	for _, src := range srcs {
		if len(src.Handle) > w.handleWidthMax {
			w.handleWidthMax = len(src.Handle)
		}
	}
	return nil
}

// Result implements output.PingWriter.
func (w *PingWriter) Result(src *source.Source, d time.Duration, err error) error {
	w.pr.Handle.Fprintf(w.out, "%-"+strconv.Itoa(w.handleWidthMax)+"s", src.Handle)
	w.pr.Duration.Fprintf(w.out, "%10s  ", d.Truncate(time.Millisecond).String())

	// The ping result is one of:
	// - success
	// - timeout
	// - some other error

	switch {
	case err == nil:
		w.pr.Success.Fprintf(w.out, "pong")

	case errors.Is(err, context.DeadlineExceeded):
		w.pr.Error.Fprintf(w.out, "fail")
		// Special rendering for timeout error
		fmt.Fprint(w.out, "  timeout exceeded")

	default: // err other than timeout err
		w.pr.Error.Fprintf(w.out, "fail")
		fmt.Fprintf(w.out, "  %s", err)
	}

	fmt.Fprintf(w.out, "\n")
	return nil
}

func (w *PingWriter) Close() error {
	return nil
}
