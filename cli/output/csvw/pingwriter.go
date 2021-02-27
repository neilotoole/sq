package csvw

import (
	"context"
	"encoding/csv"
	"io"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/cli/output"

	"github.com/neilotoole/sq/libsq/source"
)

// NewPingWriter returns a new instance.
func NewPingWriter(out io.Writer, sep rune) output.PingWriter {
	csvw := csv.NewWriter(out)
	csvw.Comma = sep
	return &pingWriter{csvw: csvw}
}

// pingWriter implements out.pingWriter.
type pingWriter struct {
	csvw *csv.Writer
}

// Open implements output.PingWriter.
func (p *pingWriter) Open(srcs []*source.Source) {
}

// Result implements output.PingWriter.
func (p *pingWriter) Result(src *source.Source, d time.Duration, err error) {
	rec := make([]string, 3)
	rec[0] = src.Handle
	rec[1] = d.Truncate(time.Millisecond).String()
	if err != nil {
		if err == context.DeadlineExceeded {
			rec[2] = "timeout exceeded"
		} else {
			rec[2] = err.Error()
		}
	} else {
		rec[2] = "pong"
	}

	_ = p.csvw.Write(rec)
	p.csvw.Flush()
}

// Close implements output.PingWriter.
func (p *pingWriter) Close() error {
	p.csvw.Flush()
	return errz.Err(p.csvw.Error())
}
