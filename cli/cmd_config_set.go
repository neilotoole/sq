package cli

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/cli/run"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set OPTION VALUE",
		RunE:              execConfigSet,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeConfigSet,
		Short:             "Set config value",
		Long: `Set option value, for base config, or for a specified source.
To get help for an individual option, use "sq config set OPTION --help".
Use "sq config get -v" to list available options.`,
		Example: `  # Set base output format
  $ sq config set format json

  # Set base max DB connections
  $ sq config set conn.max-open 10

  # Set max DB connections for source @sakila
  $ sq config set --src @sakila conn.max-open 50

  # Delete an option (resets to default value)
  $ sq config set -D conn.max-open

  # Help for an individual option
  $ sq config set conn.max-open --help`,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	cmd.Flags().String(flag.ConfigSrc, "", flag.ConfigSrcUsage)
	panicOn(cmd.RegisterFlagCompletionFunc(flag.ConfigSrc, completeHandle(1)))

	cmd.Flags().BoolP(flag.ConfigDelete, flag.ConfigDeleteShort, false, flag.ConfigDeleteUsage)

	cmd.SetHelpFunc(helpConfigSet)

	return cmd
}

func execConfigSet(cmd *cobra.Command, args []string) error {
	log := logFrom(cmd)
	ru, ctx := run.FromContext(cmd.Context()), cmd.Context()

	o := ru.Config.Options

	opt := ru.OptionsRegistry.Get(args[0])
	if opt == nil {
		return errz.Errorf("invalid config key: %s", args[0])
	}

	var src *source.Source
	if cmdFlagChanged(cmd, flag.ConfigSrc) {
		handle, err := cmd.Flags().GetString(flag.ConfigSrc)
		if err != nil {
			return errz.Err(err)
		}

		src, err = ru.Config.Collection.Get(handle)
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

		if err := ru.ConfigStore.Save(ctx, ru.Config); err != nil {
			return err
		}

		return ru.Writers.Config.UnsetOption(opt)
	}

	if len(args) < 2 {
		return errz.Errorf("accepts 2 args, received %d", len(args))
	}

	o2 := options.Options{}
	o2[opt.Key()] = args[1]
	var err error

	if o2, err = opt.Process(o2); err != nil {
		return err
	}

	o[opt.Key()] = o2[opt.Key()]
	if err = ru.ConfigStore.Save(ctx, ru.Config); err != nil {
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

	return ru.Writers.Config.SetOption(o, opt)
}

// completeConfigSet is the completion func for "config set".
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

// helpConfigSet is a custom help function for "sq config set KEY".
func helpConfigSet(cmd *cobra.Command, arr []string) {
	hlp := cmd.Parent().HelpFunc()
	ru := run.FromContext(cmd.Context())
	if ru == nil || ru.OptionsRegistry == nil || len(arr) < 4 {
		hlp(cmd, arr)
		return
	}

	// Flags are not parsed yet. Do a poor man's parsing to
	// eliminate noise.
	a := lo.Reject(arr, func(item string, index int) bool {
		if strings.HasPrefix(item, "-") {
			return true
		}

		if index-1 > 0 {
			if arr[index-1] == "--src" {
				return true
			}
		}

		return false
	})

	key := a[len(a)-1]
	opt := ru.OptionsRegistry.Get(key)
	if opt == nil {
		hlp(cmd, arr)
		return
	}

	const tpl = `Set config value.

Usage:
  sq config set %s %v

%s

See docs for more: https://sq.io/docs/config
`

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, tpl, key, opt.DefaultAny(), opt.Help())
}
