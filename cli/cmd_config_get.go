package cli

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/spf13/cobra"
)

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

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	return cmd
}

func execConfigGet(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())

	o := rc.Config.Options
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

		o = src.Options
	}

	if len(args) == 0 {
		return rc.writers.configw.Options(rc.OptionsRegistry, o)
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
	o2 := options.Options{}
	if v, ok := o[opt.Key()]; ok {
		o2[opt.Key()] = v
	}

	return rc.writers.configw.Options(reg, o2)
}
