package cli

import (
	"path/filepath"

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

		Example: `  # Print config location
  $ sq config location`,
	}

	return cmd
}

func newConfigLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location",
		Short: "Print config location",
		Long:  "Print config location. Use --verbose for more detail.",
		Args:  cobra.ExactArgs(0),
		RunE:  execConfigLocation,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	return cmd
}

func execConfigLocation(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	path := filepath.Dir(rc.ConfigStore.Location())
	var origin string
	if store, ok := rc.ConfigStore.(*config.YAMLFileStore); ok {
		origin = store.PathOrigin
	}

	return rc.writers.configw.Location(path, origin)
}
