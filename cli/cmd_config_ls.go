package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/spf13/cobra"
)

func newConfigListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [--src @HANDLE]",
		Short: "Show config",
		Long: `Show config. By default, only explicitly set options are shown.
Use the -v flag to see all options. When flag --src is provided, show config
just for that source.`,
		Args: cobra.NoArgs,
		RunE: execConfigList,
		Example: `  # Show base config
  $ sq config ls

  # Also show defaults and unset options
  $ sq config ls -v

  # Show all config in YAML
  $ sq config ls -yv

  # Show config for source
  $ sq config ls --src @sakila

  # Show config for source, including defaults and unset options
  $ sq config ls --src @sakila -v

  # Show config for active source
  $ sq config ls --src @active`,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	return cmd
}

func execConfigList(cmd *cobra.Command, _ []string) error {
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
		opts := filterOptionsForSrc(src.Type, reg.Opts()...)
		reg = &options.Registry{}
		reg.Add(opts...)
	}

	return rc.writers.configw.Options(reg, o)
}
