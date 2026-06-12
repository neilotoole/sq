package jsonw

import (
	"io"
	"time"

	"github.com/neilotoole/jsoncolor"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.PingWriter = (*pingWriter)(nil)

// NewPingWriter returns JSON impl of output.PingWriter.
func NewPingWriter(out io.Writer, pr *output.Printing) output.PingWriter {
	return &pingWriter{out: out, pr: pr}
}

type pingWriter struct {
	out io.Writer
	pr  *output.Printing
}

// Open implements output.PingWriter.
func (p pingWriter) Open(_ []*source.Source) error {
	return nil
}

// Result implements output.PingWriter.
func (p pingWriter) Result(src *source.Source, d time.Duration, err error) error {
	// Redact Location when Printing instructs us to. The embedded
	// *source.Source serializes Location verbatim, so without this guard
	// `sq ping --json --expand` would emit any resolved password in
	// plaintext. Mirror the pattern from sourcewriter.go: clone, then
	// redact, so the caller's source is not mutated.
	if p.pr.Redact {
		src = src.Clone()
		src.Location = src.RedactedLocation()
	}

	r := struct { //nolint:govet // field alignment
		*source.Source
		Pong     bool          `json:"pong"`
		Duration time.Duration `json:"duration"`
		Error    string        `json:"error,omitempty"`
	}{
		Source:   src,
		Pong:     err == nil,
		Duration: d,
	}

	if err != nil {
		// Concise human form when available, matching the error
		// rendering of error.format=json.
		r.Error = errz.HumanMessage(err)
	}

	enc := jsoncolor.NewEncoder(p.out)
	enc.SetColors(newJSONColorPalette(p.pr))
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	return errz.Err(enc.Encode(r))
}

// Close implements output.PingWriter.
func (p pingWriter) Close() error {
	return nil
}
