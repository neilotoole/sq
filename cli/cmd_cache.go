package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Args:  cobra.NoArgs,
		Short: "Manage cache",
		Long:  `Manage cache.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		Example: `  # Print cache location.
  $ sq cache location

  # Show cache info.
  $ sq cache stat

  $ sq cache enable

  $ sq cache disable
  # Disable cache for an individual source.
  $ sq cache disable @sakila

  $ sq cache clear
  $ sq cache clear @sakila

  # Print tree view of cache dir.
  $ sq cache tree`,
	}

	return cmd
}

func newCacheLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location",
		Short: "Print cache location",
		Long:  "Print cache location.",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ru := run.FromContext(cmd.Context())
			return ru.Writers.Config.CacheLocation(ru.Files.CacheDir())
		},
		Example: `  $ sq cache location
  /Users/neilotoole/Library/Caches/sq/f36ac695`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func newCacheStatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stat",
		Short: "Show cache info",
		Long:  "Show cache info, including location and size.",
		Args:  cobra.ExactArgs(0),
		RunE:  execCacheStat,
		Example: `  $ sq cache stat
  /Users/neilotoole/Library/Caches/sq/f36ac695  enabled  (472.8MB)`,
	}

	cmdMarkRequiresConfigLock(cmd)
	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execCacheStat(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	dir := ru.Files.CacheDir()

	size, err := ioz.DirSize(dir)
	if err != nil {
		lg.FromContext(cmd.Context()).Warn("Could not determine cache size",
			lga.Path, dir, lga.Err, err)
		size = -1 // -1 tells the printer that the size is unavailable.
	}

	enabled := driver.OptIngestCache.Get(ru.Config.Options)
	return ru.Writers.Config.CacheStat(dir, enabled, size)
}

func newCacheClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "clear [@HANDLE]",
		Short:             "Clear cache",
		Long:              "Clear cache for source or entire cache.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE:              execCacheClear,
		Example: `  # Clear entire cache
  $ sq cache clear

  # Clear cache for @sakila
  $ sq cache clear @sakila`,
	}

	cmdMarkRequiresConfigLock(cmd)
	return cmd
}

func execCacheClear(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	if len(args) == 0 {
		return ru.Files.CacheClearAll(ctx)
	}

	src, err := ru.Config.Collection.Get(args[0])
	if err != nil {
		return err
	}

	unlock, err := ru.Files.CacheLockAcquire(ctx, src)
	if err != nil {
		return err
	}
	defer unlock()

	return ru.Files.CacheClearSource(ctx, src, true)
}

func newCacheTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Print tree view of cache dir",
		Long:  "Print tree view of cache dir.",
		Args:  cobra.ExactArgs(0),
		RunE:  execCacheTree,
		Example: `  # Print cache tree
  $ sq cache tree

  # Print cache tree with sizes
  $ sq cache tree --size`,
	}

	cmdMarkRequiresConfigLock(cmd)
	_ = cmd.Flags().BoolP(flag.CacheTreeSize, flag.CacheTreeSizeShort, false, flag.CacheTreeSizeUsage)
	return cmd
}

func execCacheTree(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	cacheDir := ru.Files.CacheDir()
	if !ioz.DirExists(cacheDir) {
		return nil
	}

	showSize := cmdFlagBool(cmd, flag.CacheTreeSize)
	return ioz.PrintTree(ru.Out, cacheDir, showSize, !ru.Writers.PrOut.IsMonochrome())
}

func newCacheEnableCmd() *cobra.Command { //nolint:dupl
	cmd := &cobra.Command{
		Use:               "enable [@HANDLE]",
		Short:             "Enable caching",
		Long:              `Enable caching by default or for a specific source.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE: func(cmd *cobra.Command, args []string) error {
			ru := run.FromContext(cmd.Context())
			var o options.Options

			if len(args) == 0 {
				o = ru.Config.Options
			} else {
				src, err := ru.Config.Collection.Get(args[0])
				if err != nil {
					return err
				}
				if src.Options == nil {
					src.Options = options.Options{}
				}
				o = src.Options
			}

			o[driver.OptIngestCache.Key()] = true
			return ru.ConfigStore.Save(cmd.Context(), ru.Config)
		},
		Example: `  # Enable caching by default
  $ sq cache enable

  # Enable caching for a particular source
  $ sq cache enable @sakila`,
	}

	cmdMarkRequiresConfigLock(cmd)
	return cmd
}

func newCacheDisableCmd() *cobra.Command { //nolint:dupl
	cmd := &cobra.Command{
		Use:               "disable [@HANDLE]",
		Short:             "Disable caching",
		Long:              `Disable caching by default or for a specific source.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE: func(cmd *cobra.Command, args []string) error {
			ru := run.FromContext(cmd.Context())
			var o options.Options

			if len(args) == 0 {
				o = ru.Config.Options
			} else {
				src, err := ru.Config.Collection.Get(args[0])
				if err != nil {
					return err
				}
				if src.Options == nil {
					src.Options = options.Options{}
				}
				o = src.Options
			}

			o[driver.OptIngestCache.Key()] = false
			return ru.ConfigStore.Save(cmd.Context(), ru.Config)
		},
		Example: `  # Disable caching by default
  $ sq cache disable

  # Disable caching for a particular source
  $ sq cache disable @sakila`,
	}

	cmdMarkRequiresConfigLock(cmd)
	return cmd
}
