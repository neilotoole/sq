package cli

import (
	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rm @HANDLE|GROUP",

		Short: "Remove data source or group",
		Long: `Remove data source or group. Removing a group removes
all sources in that group. On return, the active source or active group
may have changed, if that source or group was removed.`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              execRemove,
		ValidArgsFunction: completeHandleOrGroup,
		Example: `  # Remove @sakila source
  $ sq rm @sakila_db

  # Remove multiple data sources
  $ sq rm @sakila/pg @sakila_my

  # Remove the "prod" group (and all its children)
  $ sq rm prod

  # Remove a mix of sources and groups
  $ sq rm @staging/sakila_db @staging/backup_db dev`,
	}

	cmdMarkRequiresConfigLock(cmd)
	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	addOptionFlag(cmd.Flags(), OptCompact)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	return cmd
}

// execRemove removes sources and groups. The elements of
// args can be a handle, or a group.
func execRemove(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	cfg, coll := ru.Config, ru.Config.Collection

	args = lo.Uniq(args)
	var removed []*source.Source
	for _, arg := range args {
		switch {
		case source.IsValidHandle(arg):
			if source.Contains(removed, arg) {
				// removed may already contain the handle
				// by virtue of its group having been removed.
				continue
			}

			src, err := coll.Get(arg)
			if err != nil {
				return err
			}

			err = coll.Remove(src.Handle)
			if err != nil {
				return err
			}
			removed = append(removed, src)
		case source.IsValidGroup(arg):
			removedViaGroup, err := coll.RemoveGroup(arg)
			if err != nil {
				return err
			}

			removed = append(removed, removedViaGroup...)
		default:
			return errz.Errorf("invalid arg: %s", arg)
		}
	}

	if err := ru.ConfigStore.Save(cmd.Context(), cfg); err != nil {
		return err
	}
	lo.Uniq(removed)
	source.Sort(removed)
	return ru.Writers.Source.Removed(removed...)
}
