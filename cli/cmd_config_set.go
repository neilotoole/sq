package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/spf13/cobra"
)

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set",
		RunE:              execConfigSet,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeConfigSet,
		Short:             "Set config value",
		Long:              `Set config value.`,
		Example: `  # Set default output format
  $ sq config set format json

  # Set default max DB connections
  $ sq config set conn.max-open 10`,
	}

	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigSet(cmd *cobra.Command, args []string) error {
	rc, ctx := RunContextFrom(cmd.Context()), cmd.Context()

	opt := rc.OptionsRegistry.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid config key: %s", args[0])
	}

	o := options.Options{}
	o[opt.Key()] = args[1]
	var err error

	if o, err = opt.Process(o); err != nil {
		return err
	}

	rc.Config.Options[opt.Key()] = o[opt.Key()]
	if err = rc.ConfigStore.Save(ctx, rc.Config); err != nil {
		return err
	}

	lg.From(ctx).Info("Set config value", lga.Val, o)

	return rc.writers.configw.SetOption(rc.OptionsRegistry, o, opt)
}

func completeConfigSet(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeOptKey(cmd, args, toComplete)
	case 1:
		return completeOptValue(cmd, args, toComplete)
	default:
		// Maximum of two args
		return nil, cobra.ShellCompDirectiveError
	}
}
