package cli

import (
	"github.com/neilotoole/sq/cli/config"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage config",
		Long:  "Manage config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},

		Example: `  # Print config dir
  $ sq config dir`,
	}

	return cmd
}

func newConfigDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dir",
		Short: "Print config dir",
		Long:  "Print config dir. Use --verbose for more detail.",
		Args:  cobra.ExactArgs(0),
		RunE:  execConfigDir,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	return cmd
}

func execConfigDir(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	path := rc.ConfigStore.Location()
	var origin string
	if store, ok := rc.ConfigStore.(*config.YAMLFileStore); ok {
		origin = store.PathOrigin
	}

	return rc.writers.configw.Dir(path, origin)
}
