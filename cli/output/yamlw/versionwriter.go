package yamlw

import (
	"io"

	"github.com/goccy/go-yaml/printer"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/output"
)

var _ output.VersionWriter = (*versionWriter)(nil)

// versionWriter implements output.VersionWriter for JSON.
type versionWriter struct {
	p   printer.Printer
	out io.Writer
	pr  *output.Printing
}

// NewVersionWriter returns a new output.VersionWriter instance
// that outputs version info in JSON.
func NewVersionWriter(out io.Writer, pr *output.Printing) output.VersionWriter {
	return &versionWriter{p: newPrinter(pr), out: out, pr: pr}
}

// Version implements output.VersionWriter.
func (w *versionWriter) Version(bi buildinfo.Info, latestVersion string, hi hostinfo.Info) error {
	type cliBuildInfo struct {
		Version       string        `json:"version" yaml:"version"`
		Commit        string        `json:"commit,omitempty" yaml:"commit,omitempty"`
		Timestamp     string        `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
		LatestVersion string        `json:"latest_version" yaml:"latest_version"`
		Host          hostinfo.Info `json:"host" yaml:"host"`
	}

	cbi := cliBuildInfo{
		Version:       bi.Version,
		Commit:        bi.Commit,
		Timestamp:     w.pr.FormatDatetime(bi.Timestamp),
		LatestVersion: latestVersion,
		Host:          hi,
	}

	return writeYAML(w.out, w.p, cbi)
}
