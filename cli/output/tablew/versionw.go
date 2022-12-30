package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/output"
	"golang.org/x/mod/semver"
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

func (w *versionWriter) Version(bi buildinfo.BuildInfo, latestVersion string) error {
	fmt.Fprintf(w.out, "sq %s", bi.Version)

	// Only print more if --verbose is set.
	if !w.fm.Verbose {
		fmt.Fprintln(w.out)
		return nil
	}

	if len(bi.Commit) > 0 {
		fmt.Fprint(w.out, "    ")
		w.fm.Faint.Fprint(w.out, "#"+bi.Commit)
	}

	if len(bi.Timestamp) > 0 {
		fmt.Fprint(w.out, "    ")
		w.fm.Faint.Fprint(w.out, bi.Timestamp)
	}

	showUpdate := semver.Compare(latestVersion, bi.Version) > 0
	if showUpdate {
		fmt.Fprint(w.out, "    ")
		w.fm.Faint.Fprint(w.out, "Update available: ")
		w.fm.Number.Fprint(w.out, latestVersion)
	}

	fmt.Fprintln(w.out)
	return nil
}
