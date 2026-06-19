package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Useful database actions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		Example: `  # TBD`,
	}

	return cmd
}

// resolveToolCmdSource resolves any ${scheme:path} placeholders in
// src.Location, returning a clone if resolution occurred. The db commands
// construct external tool invocations (pg_dump etc.) directly from
// src.Location, bypassing Grips.doOpen, which performs this resolution for
// the query path. Every db command must call this before handing src to a
// tool command builder: forgetting it hands the tool the literal
// placeholder text.
func resolveToolCmdSource(cmd *cobra.Command, src *source.Source) (*source.Source, error) {
	return driver.ResolveSourceSecrets(cmd.Context(), src)
}
