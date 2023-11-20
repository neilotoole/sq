package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
)

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show config option",
		Long: `Show config option. By default, only explicitly set options are shown.
Use the -v flag to see all options. When flag --src is provided, show config
just for that source.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeOptKey,
		RunE:              execConfigGet,
		Example: `  # Show individual base option
  $ sq config get conn.max-open

  # Show more detail, in YAML
  $ sq config get conn.max-open -yv

  # Show option for src
  $ sq config get --src @sakila conn.max-open`,
	}

	addTextFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	return cmd
}

func execConfigGet(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())

	o := ru.Config.Options
	reg := ru.OptionsRegistry

	if cmdFlagChanged(cmd, flag.ConfigSrc) {
		handle, err := cmd.Flags().GetString(flag.ConfigSrc)
		if err != nil {
			return errz.Err(err)
		}

		src, err := ru.Config.Collection.Get(handle)
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

	// Print just a single option, e.g.
	//  $ sq config get conn.max-open
	opt := reg.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid option key: %s", args[0])
	}

	return ru.Writers.Config.Opt(o, opt)
}
