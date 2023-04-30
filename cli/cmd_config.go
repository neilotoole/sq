package cli

import (
	"github.com/neilotoole/sq/cli/config/yamlstore"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Args:  cobra.NoArgs,
		Short: "Manage config",
		Long:  "Manage config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: `  # Print config location
  $ sq config location

  # Show default options
  $ sq config get

  # Edit default options
  $ sq config edit

  # Edit config for source
  $ sq config edit @sakila`,
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
	if store, ok := rc.ConfigStore.(*yamlstore.Store); ok {
		origin = store.PathOrigin
	}

	return rc.writers.configw.Location(path, origin)
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "Show config",
		Long:              "Show config. In table output format, use --verbose to see defaults.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeOptKey,
		RunE:              execConfigGet,
		Example: `  # Show all config
  $ sq config get

  # Also show defaults
  $ sq config get -v

  # Show individual option
  $ sq config get conn.max-open

  # Show config for source
  # sq config get --src @sakila`,
	}

	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigGet(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())

	if len(args) == 0 {
		return rc.writers.configw.Options(rc.OptionsRegistry, rc.Config.Options)
	}

	// Print just a single option, e.g.
	//  $ sq config get conn.max-open
	opt := rc.OptionsRegistry.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid option key: %s", args[0])
	}

	// A bit of a hack... create a new registry with just the desired opt.
	reg := &options.Registry{}
	reg.Add(opt)
	o := options.Options{}
	if v, ok := rc.Config.Options[opt.Key()]; ok {
		o[opt.Key()] = v
	}

	return rc.writers.configw.Options(reg, o)
}
