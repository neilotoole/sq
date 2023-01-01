package jsonw

import (
	"io"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/output"
)

var _ output.VersionWriter = (*versionWriter)(nil)

// versionWriter implements output.VersionWriter for JSON.
type versionWriter struct {
	out io.Writer
	fm  *output.Formatting
}

// NewVersionWriter returns a new output.VersionWriter instance
// that outputs version info in JSON.
func NewVersionWriter(out io.Writer, fm *output.Formatting) output.VersionWriter {
	return &versionWriter{out: out, fm: fm}
}

func (w *versionWriter) Version(info buildinfo.BuildInfo, latestVersion string) error {
	type cliBuildInfo struct {
		buildinfo.BuildInfo
		LatestVersion string `json:"latest_version"`
	}

	bi := cliBuildInfo{
		BuildInfo:     info,
		LatestVersion: latestVersion,
	}

	return writeJSON(w.out, w.fm, bi)
}
