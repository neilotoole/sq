package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/buildinfo"
)

func newVersionCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print sq version",
	}

	return cmd, execVersion
}

func execVersion(rc *RunContext, cmd *cobra.Command, args []string) error {
	version := buildinfo.Version

	// If buildinfo.Version is not set (building without ldflags),
	// then we set a dummy version.
	if version == "" {
		version = "0.0.0.dev"
	}

	rc.wrtr.fmt.Hilite.Fprintf(rc.Out, "sq %s", version)

	if len(buildinfo.Commit) > 0 {
		fmt.Fprintf(rc.Out, "                ")
		rc.wrtr.fmt.Faint.Fprintf(rc.Out, "#"+buildinfo.Commit)
	}

	if len(buildinfo.Timestamp) > 0 {
		fmt.Fprintf(rc.Out, "       ")
		rc.wrtr.fmt.Faint.Fprintf(rc.Out, buildinfo.Timestamp)
	}

	fmt.Fprintln(rc.Out)
	return nil
}
