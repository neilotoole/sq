package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/output"
	"golang.org/x/mod/semver"
)

var _ output.VersionWriter = (*versionWriter)(nil)

// versionWriter implements output.VersionWriter for text.
type versionWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewVersionWriter returns a new output.VersionWriter instance
// that outputs version info in text.
func NewVersionWriter(out io.Writer, pr *output.Printing) output.VersionWriter {
	return &versionWriter{out: out, pr: pr}
}

func (w *versionWriter) Version(bi buildinfo.BuildInfo, latestVersion string) error {
	fmt.Fprintf(w.out, "sq %s", bi.Version)

	if w.pr.Verbose {
		if len(bi.Commit) > 0 {
			fmt.Fprint(w.out, "    ")
			w.pr.Faint.Fprint(w.out, "#"+bi.Commit)
		}

		if len(bi.Timestamp) > 0 {
			fmt.Fprint(w.out, "    ")
			w.pr.Faint.Fprint(w.out, bi.Timestamp)
		}
	}

	showUpdate := semver.Compare(latestVersion, bi.Version) > 0
	if showUpdate {
		fmt.Fprint(w.out, "    ")
		w.pr.Faint.Fprint(w.out, "Update available: "+latestVersion)
	}

	fmt.Fprintln(w.out)
	return nil
}
