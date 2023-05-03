package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/spf13/cobra"
)

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show config",
		Long: `Show config. By default, only explicitly set options are shown.
Use the --verbose flag (in text output format) to see all options.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeOptKey,
		RunE:              execConfigGet,
		Example: `  # Show base config
  $ sq config get

  # Also show defaults and unset options.
  $ sq config get -v

  # Show individual option
  $ sq config get conn.max-open

  # Show config for source
  $ sq config get --src @sakila

  # Show config for source, including defaults and unset options.
  $ sq config get --src @sakila -v

  # Show individual option for src
  $ sq config get --src @sakila conn.max-open

  # Show config for active source
  $ sq config get --src @active`,
	}

	cmd.Flags().BoolP(flag.Text, flag.TextShort, false, flag.TextUsage)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	return cmd
}

func execConfigGet(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())

	o := rc.Config.Options
	reg := rc.OptionsRegistry

	if cmdFlagChanged(cmd, flag.ConfigSrc) {
		handle, err := cmd.Flags().GetString(flag.ConfigSrc)
		if err != nil {
			return errz.Err(err)
		}

		src, err := rc.Config.Collection.Get(handle)
		if err != nil {
			return err
		}

		o = src.Options
		if o == nil {
			o = options.Options{}
		}

		// Create a new registry that only contains Opts applicable
		// to this source.
		opts := filterOptionsForSrc(src, reg.Opts()...)
		reg = &options.Registry{}
		reg.Add(opts...)
	}

	if len(args) == 0 {
		return rc.writers.configw.Options(reg, o)
	}

	// Print just a single option, e.g.
	//  $ sq config get conn.max-open
	opt := reg.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid option key: %s", args[0])
	}

	// A bit of a hack... create a new registry with just the desired opt.
	reg2 := &options.Registry{}
	reg2.Add(opt)
	o2 := options.Options{}
	if v, ok := o[opt.Key()]; ok {
		o2[opt.Key()] = v
	}

	return rc.writers.configw.Options(reg2, o2)
}
