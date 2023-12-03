package cli

import (
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
)

func newConfigCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Args:  cobra.NoArgs,
		Short: "Manage cache",
		Long:  `Manage cache.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		// FIXME: add examples
		//	Example: `  # Print config location
		//$ sq config location
		//
		//# Show base config
		//$ sq config ls
		//
		//# Show base config including unset and default values.
		//$ sq config ls -v
		//
		//# Show base config in maximum detail (YAML format)
		//$ sq config ls -yv
		//
		//# Get base value of an option
		//$ sq config get format
		//
		//# Get source-specific value of an option
		//$ sq config get --src @sakila conn.max-open
		//
		//# Set base option value
		//$ sq config set format json
		//
		//# Set source-specific option value
		//$ sq config set --src @sakila conn.max-open 50
		//
		//# Help for an option
		//$ sq config set format --help
		//
		//# Edit base config in $EDITOR
		//$ sq config edit
		//
		//# Edit config for source in $EDITOR
		//$ sq config edit @sakila
		//
		//# Delete option (reset to default value)
		//$ sq config set -D log.level`,
	}

	return cmd
}

func newConfigCacheLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "location",
		Aliases: []string{"loc"},
		Short:   "Print cache location",
		Long:    "Print cache location.",
		Args:    cobra.ExactArgs(0),
		RunE:    execConfigCacheLocation,
		Example: `  $ sq config cache location
  /Users/neilotoole/Library/Caches/sq`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigCacheLocation(cmd *cobra.Command, _ []string) error {
	dir := source.CacheDirPath()
	ru := run.FromContext(cmd.Context())
	return ru.Writers.Config.CacheLocation(dir)
}

func newConfigCacheInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Print cache info",
		Long:  "Print cache info.",
		Args:  cobra.ExactArgs(0),
		RunE:  execConfigCacheInfo,
		Example: `  $ sq config cache info
  /Users/neilotoole/Library/Caches/sq (1.2MB)`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execConfigCacheInfo(cmd *cobra.Command, _ []string) error {
	dir := source.CacheDirPath()
	ru := run.FromContext(cmd.Context())
	size, err := ioz.DirSize(dir)
	if err != nil {
		lg.FromContext(cmd.Context()).Warn("Could not determine cache size",
			lga.Path, dir, lga.Err, err)
		size = -1
	}

	return ru.Writers.Config.CacheInfo(dir, size)
}
