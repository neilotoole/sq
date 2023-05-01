package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set",
		RunE:              execConfigSet,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeConfigSet,
		Short:             "Set config value",
		Long:              `Set config value globally, or for a specific source.`,
		Example: `  # Set default output format
  $ sq config set format json

  # Set default max DB connections
  $ sq config set conn.max-open 10

  # Set max DB connections for source @sakila
  $ sq config set --src @sakila conn.max-open 50`,
	}

	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))
	return cmd
}

func execConfigSet(cmd *cobra.Command, args []string) error {
	rc, ctx := RunContextFrom(cmd.Context()), cmd.Context()

	o := rc.Config.Options

	opt := rc.OptionsRegistry.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid config key: %s", args[0])
	}

	var src *source.Source
	if cmdFlagChanged(cmd, flag.ConfigSrc) {
		handle, err := cmd.Flags().GetString(flag.ConfigSrc)
		if err != nil {
			return errz.Err(err)
		}

		src, err = rc.Config.Collection.Get(handle)
		if err != nil {
			return err
		}

		if src.Options == nil {
			src.Options = options.Options{}
		}

		o = src.Options
	}

	o2 := options.Options{}
	o2[opt.Key()] = args[1]
	var err error

	if o2, err = opt.Process(o2); err != nil {
		return err
	}

	o[opt.Key()] = o2[opt.Key()]
	if err = rc.ConfigStore.Save(ctx, rc.Config); err != nil {
		return err
	}

	if src != nil {
		lg.From(ctx).Info("Set default config value", lga.Val, o)
	} else {
		lg.From(ctx).Info("Set source config value", lga.Src, src, lga.Val, o)
	}

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
