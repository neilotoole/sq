package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Args:  cobra.NoArgs,
		Short: "Manage cache",
		Long:  `Manage cache.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		Example: `  # Print cache location.
  $ sq cache location

  # Show cache info.
  $ sq cache stat

  $ sq cache enable

  $ sq cache disable

  $ sq cache clear

  # Print tree view of cache dir.
  $ sq cache tree`,
	}

	return cmd
}

func newCacheLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "location",
		Aliases: []string{"loc"},
		Short:   "Print cache location",
		Long:    "Print cache location.",
		Args:    cobra.ExactArgs(0),
		RunE:    execCacheLocation,
		Example: `  $ sq cache location
  /Users/neilotoole/Library/Caches/sq`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execCacheLocation(cmd *cobra.Command, _ []string) error {
	dir := source.DefaultCacheDir()
	ru := run.FromContext(cmd.Context())
	return ru.Writers.Config.CacheLocation(dir)
}

func newCacheInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stat",
		Short: "Show cache info",
		Long:  "Show cache info, including location and size.",
		Args:  cobra.ExactArgs(0),
		RunE:  execCacheInfo,
		Example: `  $ sq cache stat
  /Users/neilotoole/Library/Caches/sq  enabled  (472.8MB)`,
	}

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

func execCacheInfo(cmd *cobra.Command, _ []string) error {
	dir := source.DefaultCacheDir()
	ru := run.FromContext(cmd.Context())
	size, err := ioz.DirSize(dir)
	if err != nil {
		lg.FromContext(cmd.Context()).Warn("Could not determine cache size",
			lga.Path, dir, lga.Err, err)
		size = -1 // -1 tells the printer that the size is unavailable.
	}

	enabled := driver.OptIngestCache.Get(ru.Config.Options)
	return ru.Writers.Config.CacheInfo(dir, enabled, size)
}

func newCacheClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Clear cache",
		Long:    "Clear cache. May cause issues if another sq instance is running.",
		Args:    cobra.ExactArgs(0),
		RunE:    execCacheClear,
		Example: `  $ sq cache clear`,
	}

	markCmdRequiresConfigLock(cmd)
	return cmd
}

func execCacheClear(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	return ru.Files.CacheClear(cmd.Context())
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

	_ = cmd.Flags().BoolP(flag.CacheTreeSize, flag.CacheTreeSizeShort, false, flag.CacheTreeSizeUsage)
	return cmd
}

func execCacheTree(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	cacheDir := source.DefaultCacheDir()
	if !ioz.DirExists(cacheDir) {
		return nil
	}

	showSize := cmdFlagBool(cmd, flag.CacheTreeSize)
	return ioz.PrintTree(ru.Out, cacheDir, showSize, !ru.Writers.Printing.IsMonochrome())
}

func newCacheEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable caching",
		Long: `Disable caching. This is equivalent to:

  $ sq config set ingest.cache true`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return execConfigSet(cmd, []string{driver.OptIngestCache.Key(), "true"})
		},
		Example: `  $ sq cache enable`,
	}
	markCmdRequiresConfigLock(cmd)
	return cmd
}

func newCacheDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable caching",
		Long: `Disable caching. This is equivalent to:

  $ sq config set ingest.cache false`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return execConfigSet(cmd, []string{driver.OptIngestCache.Key(), "false"})
		},
		Example: `  $ sq cache disable`,
	}

	markCmdRequiresConfigLock(cmd)
	return cmd
}
