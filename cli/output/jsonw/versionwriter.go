package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/hostinfo"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/output"
)

var _ output.VersionWriter = (*versionWriter)(nil)

// versionWriter implements output.VersionWriter for JSON.
type versionWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewVersionWriter returns a new output.VersionWriter instance
// that outputs version info in JSON.
func NewVersionWriter(out io.Writer, pr *output.Printing) output.VersionWriter {
	return &versionWriter{out: out, pr: pr}
}

// Version implements output.VersionWriter.
func (w *versionWriter) Version(bi buildinfo.BuildInfo, latestVersion string, hi hostinfo.Info) error {
	// We use a custom struct so that we can
	// control the timestamp format.
	type cliBuildInfo struct {
		Version       string        `json:"version" yaml:"version"`
		Commit        string        `json:"commit,omitempty" yaml:"commit,omitempty"`
		Timestamp     string        `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
		LatestVersion string        `json:"latest_version"`
		Host          hostinfo.Info `json:"host"`
	}

	cbi := cliBuildInfo{
		Version:       bi.Version,
		Commit:        bi.Commit,
		LatestVersion: latestVersion,
		Host:          hi,
	}

	if !bi.Timestamp.IsZero() {
		cbi.Timestamp = w.pr.FormatDatetime(bi.Timestamp)
	}

	return writeJSON(w.out, w.pr, cbi)
}
