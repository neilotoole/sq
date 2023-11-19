package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/hostinfo"

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

// Version implements output.VersionWriter.
func (w *versionWriter) Version(bi buildinfo.BuildInfo, latestVersion string, hi hostinfo.Info) error {
	var newerAvailable bool

	if latestVersion != "" {
		newerAvailable = semver.Compare(latestVersion, bi.Version) > 0
	}

	if !w.pr.Verbose {
		fmt.Fprintf(w.out, "sq %s", bi.Version)

		if newerAvailable {
			fmt.Fprint(w.out, "    ")
			w.pr.Faint.Fprint(w.out, "Update available: "+latestVersion)
		}

		fmt.Fprintln(w.out)
		return nil
	}

	fmt.Fprintf(w.out, "sq %s\n", bi.Version)

	w.pr.Faint.Fprintf(w.out, "Version:         %s\n", bi.Version)

	if bi.Commit != "" {
		w.pr.Faint.Fprintf(w.out, "Commit:          #%s\n", bi.Commit)
	}

	if !bi.Timestamp.IsZero() {
		w.pr.Faint.Fprintf(w.out, "Timestamp:       %s\n", w.pr.FormatDatetime(bi.Timestamp))
	}

	// latestVersion = ""
	w.pr.Faint.Fprint(w.out, "Latest version:  ")
	if latestVersion == "" {
		w.pr.Error.Fprintf(w.out, "unknown\n")
	} else {
		if newerAvailable {
			w.pr.Hilite.Fprintln(w.out, latestVersion)
		} else {
			w.pr.Faint.Fprintln(w.out, latestVersion)
		}
	}

	w.pr.Faint.Fprintf(w.out, "Host:            %s %s | %s %s | %s %s\n",
		hi.Platform, hi.Arch, hi.Kernel, hi.KernelVersion, hi.Variant, hi.VariantVersion)

	// Follow GNU standards (mostly)
	// https://www.gnu.org/prep/standards/html_node/_002d_002dversion.html#g_t_002d_002dversion
	const notice = `MIT License:     https://opensource.org/license/mit
Website:         https://sq.io
Source code:     https://github.com/neilotoole/sq
Notice:          Copyright (c) 2023 Neil O'Toole`
	w.pr.Faint.Fprintln(w.out, notice)

	return nil
}
