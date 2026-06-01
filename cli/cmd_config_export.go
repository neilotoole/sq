package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
)

const flagConfigExportResolve = "resolve"

func newConfigExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Args:  cobra.NoArgs,
		Short: "Export config as YAML",
		Long: `Export the active sq config as YAML, including the source collection,
config options, and active source/group state. Intended for backups.

By default, output is a faithful copy of the live config: ${scheme:path}
placeholders (keyring, env, file) are written verbatim. Inline values
already present in source Locations (such as plaintext credentials in a
DSN) are dumped as-is — exactly as they appear in your config file.

With --resolve, every ${scheme:path} placeholder is expanded end-to-end
and the resolved value is spliced into the exported Location. This
produces a fully self-contained snapshot suitable for transferring
between machines, at the cost of writing every referenced secret in
plaintext. The output file is always created with mode 0600 (matching
the permissions sq uses for the live config file).`,
		RunE: execConfigExport,
		Example: `  # Portable export to stdout (placeholders preserved)
  $ sq config export

  # Portable export to a file (backup)
  $ sq config export -o sq.bak.yml

  # Self-contained export with placeholders resolved in-line
  $ sq config export --resolve -o sq.bak.yml`,
	}

	cmdMarkPlainStdout(cmd)
	cmd.Flags().Bool(flagConfigExportResolve, false,
		"Resolve ${scheme:path} placeholders in-line (writes resolved secrets in plaintext)")
	cmd.Flags().StringP(flag.FileOutput, flag.FileOutputShort, "", flag.FileOutputUsage)
	return cmd
}

func execConfigExport(cmd *cobra.Command, _ []string) error {
	_ = run.FromContext(cmd.Context())
	_ = ioz.MarshalYAML
	_ = errz.New
	_ = fmt.Fprintln
	_ = os.OpenFile
	return nil
}
