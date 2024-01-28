package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Args:  cobra.NoArgs,
		Short: "Manage config",
		Long:  `View and edit config.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: `  # Print config location
  $ sq config location

  # Show base config
  $ sq config ls

  # Show base config including unset and default values.
  $ sq config ls -v

  # Show base config in maximum detail (YAML format)
  $ sq config ls -yv

  # Get base value of an option
  $ sq config get format

  # Get source-specific value of an option
  $ sq config get --src @sakila conn.max-open

  # Set base option value
  $ sq config set format json

  # Set source-specific option value
  $ sq config set --src @sakila conn.max-open 50

  # Help for an option
  $ sq config set format --help

  # Edit base config in $EDITOR
  $ sq config edit

  # Edit config for source in $EDITOR
  $ sq config edit @sakila

  # Delete option (reset to default value)
  $ sq config set -D log.level`,
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
		Example: `  # Print config location
  $ sq config location
  /Users/neilotoole/.config/sq

  # Print location, also show origin (flag, env, default)
  $ sq config location -v
  /Users/neilotoole/.config/sq
  Origin: env`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigLocation(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	path := ru.ConfigStore.Location()
	var origin string
	if store, ok := ru.ConfigStore.(*yamlstore.Store); ok {
		origin = store.PathOrigin
	}

	return ru.Writers.Config.Location(path, origin)
}
