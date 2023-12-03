package cli

import (
	"io"
	"os"
	"path/filepath"

	"github.com/a8m/tree"
	"github.com/a8m/tree/ostree"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
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
		Example: `  $ sq cache location
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
		Short: "Show cache info",
		Long:  "Show cache info, including location and size.",
		Args:  cobra.ExactArgs(0),
		RunE:  execConfigCacheInfo,
		Example: `  $ sq cache info
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

	enabled := driver.OptIngestCache.Get(ru.Config.Options)
	return ru.Writers.Config.CacheInfo(dir, enabled, size)
}

func newConfigCacheClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Clear cache",
		Long:    "Clear cache. May cause issues if another sq instance is running.",
		Args:    cobra.ExactArgs(0),
		RunE:    execConfigCacheClear,
		Example: `  $ sq cache clear`,
	}

	return cmd
}

func execConfigCacheClear(cmd *cobra.Command, _ []string) error {
	log := lg.FromContext(cmd.Context())
	cacheDir := source.CacheDirPath()
	if !ioz.DirExists(cacheDir) {
		return nil
	}

	// Instead of directly deleting the existing cache dir, we first
	// move it to /tmp, and then try to delete it. This should probably
	// help with the situation where another sq instance has an open pid
	// lock in the cache dir.
	tmpLoc := filepath.Join(os.TempDir(), "sq", "dead_cache_"+stringz.Uniq8())
	if err := os.Rename(cacheDir, tmpLoc); err != nil {
		return errz.Wrap(err, "clear cache: relocate")
	}

	deleteErr := os.RemoveAll(tmpLoc)
	if deleteErr != nil {
		log.Warn("Could not delete relocated cache dir", lga.Path, tmpLoc, lga.Err, deleteErr)
	}

	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return errz.Wrap(err, "clear cache")
	}

	return nil
}

func newConfigCacheTreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tree",
		Short:   "Print tree view of cache dir",
		Long:    "Print tree view of cache dir.",
		Args:    cobra.ExactArgs(0),
		RunE:    execConfigCacheTree,
		Example: `  $ sq cache tree`,
	}

	return cmd
}

func execConfigCacheTree(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	cacheDir := source.CacheDirPath()
	if !ioz.DirExists(cacheDir) {
		return nil
	}
	return printFileTree(ru.Out, cacheDir)
}

func printFileTree(w io.Writer, loc string) error {
	opts := &tree.Options{
		Fs:      new(ostree.FS),
		OutFile: w,
		All:     false,
		// DirsOnly:   false,
		// FullPath:   false,
		// IgnoreCase: false,
		// FollowLink: false,
		// DeepLevel:  0,
		// Pattern:    "",
		// IPattern:   "",
		// MatchDirs:  false,
		// Prune:      false,
		// ByteSize:   false,
		// UnitSize:   true,
		// FileMode:   false,
		// ShowUid:    false,
		// ShowGid:    false,
		// LastMod:    false,
		// Quotes:     false,
		// Inodes:     false,
		// Device:     false,
		// NoSort:     false,
		// VerSort:    false,
		// ModSort:    false,
		// DirSort:    false,
		// NameSort:   false,
		// SizeSort:   false,
		// CTimeSort:  false,
		// ReverSort:  false,
		// NoIndent:   false,
		Colorize: true,
		// Color:    nil,
	}

	inf := tree.New(loc)
	_, _ = inf.Visit(opts)
	inf.Print(opts)
	return nil
}
