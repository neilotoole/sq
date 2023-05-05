package cli

import (
	"fmt"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set",
		RunE:              execConfigSet,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeConfigSet,
		Short:             "Set config value",
		Long: `Set config value globally, or for a specific source.
Use "sq config get -v" to see available options.`,
		Example: `  # Set default output format
  $ sq config set format json

  # Set default max DB connections
  $ sq config set conn.max-open 10

  # Set max DB connections for source @sakila
  $ sq config set --src @sakila conn.max-open 50

  # Delete an option (reset to default value)
  $ sq config set -D conn.max-open`,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	cmd.Flags().Bool(flag.Pretty, true, flag.PrettyUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	cmd.Flags().BoolP(flag.ConfigDelete, flag.ConfigDeleteShort, false, flag.ConfigDeleteUsage)

	return cmd
}

func execConfigSet(cmd *cobra.Command, args []string) error {
	log := logFrom(cmd)
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

	if cmdFlagChanged(cmd, flag.ConfigDelete) {
		if len(args) > 1 {
			return errz.Errorf("accepts 1 arg when used with --%s flag", flag.ConfigDelete)
		}

		delete(o, opt.Key())
		if src == nil {
			log.Info("Unset base config value", lga.Key, opt.Key())
		} else {
			log.Info("Unset source config value", lga.Src, src, lga.Key, opt.Key())
		}

		if err := rc.ConfigStore.Save(ctx, rc.Config); err != nil {
			return err
		}

		return rc.writers.configw.UnsetOption(opt)
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

	if src == nil {
		log.Info(
			"Set base config value",
			lga.Key, opt.Key(),
			lga.Val, o[opt.Key()],
		)
	} else {
		log.Info(
			"Set source config value",
			lga.Key, opt.Key(),
			lga.Src, src,
			lga.Val, o,
		)
	}

	return rc.writers.configw.SetOption(rc.OptionsRegistry, o, opt)
}

func completeConfigSet(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return completeOptKey(cmd, args, toComplete)

	case 1:
		if cmdFlagChanged(cmd, flag.ConfigDelete) {
			logFrom(cmd).Warn(fmt.Sprintf("No 2nd arg when using --%s flag", flag.ConfigDelete))
			return nil, cobra.ShellCompDirectiveError
		}

		return completeOptValue(cmd, args, toComplete)
	default:
		// Maximum of two args
		return nil, cobra.ShellCompDirectiveError
	}
}
