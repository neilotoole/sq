package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/buildinfo"
)

func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print sq version",
		RunE:  execVersion,
	}

	return cmd
}

func execVersion(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	rc.writers.fmt.Hilite.Fprintf(rc.Out, "sq %s", buildinfo.Version)

	if len(buildinfo.Commit) > 0 {
		fmt.Fprint(rc.Out, "    ")
		rc.writers.fmt.Faint.Fprint(rc.Out, "#"+buildinfo.Commit)
	}

	if len(buildinfo.Timestamp) > 0 {
		fmt.Fprint(rc.Out, "    ")
		rc.writers.fmt.Faint.Fprint(rc.Out, buildinfo.Timestamp)
	}

	fmt.Fprintln(rc.Out)
	return nil
}
