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
		Example: `  # Print config location
  $ sq config location

  # Show config
  $ sq config get

  # Edit config
  $ sq config edit`,
	}

	return cmd
}

func newConfigLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "location",
		Aliases: []string{"loc"},
		Short:   "Print config location",
		Long:    "Print config location. Use --verbose for more detail.",
		Args:    cobra.ExactArgs(0),
		RunE:    execConfigLocation,
		Example: `  # Print config location
  $ sq config location
  /Users/neilotoole/.config/sq

  # Print location, also show origin (flag, env, default)
  $ sq config location -v
  /Users/neilotoole/.config/sq
  Origin: env`,
	}

	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigLocation(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	path := rc.ConfigStore.Location()
	var origin string
	if store, ok := rc.ConfigStore.(*config.YAMLFileStore); ok {
		origin = store.PathOrigin
	}

	return rc.writers.configw.Location(path, origin)
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show config",
		Long:  "Show config.",
		Args:  cobra.ExactArgs(0),
		RunE:  execConfigGet,
	}

	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigGet(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())

	opts := rc.Config.Options
	return rc.writers.configw.Options(&opts)
}
