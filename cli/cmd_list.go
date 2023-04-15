package cli

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls [GROUP]",
		Short: "List sources and groups",
		Long: `List data sources for active group. If GROUP is specified, list for only that group.
If --group is set, list groups instead of sources.

The source list includes all descendants of the group: direct children, and also
any further descendants.
`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGroup(1),
		RunE:              execList,
		Example: `  # List sources in active group
  $ sq ls

  # List sources in the "prod" group
  $ sq ls prod

  # List sources in the root group (will list all sources)
  $ sq ls /

  # List groups instead of sources
  $ sq ls -g`,
	}

	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagListGroups, flagListGroupsShort, true, flagListGroupsUsage)

	return cmd
}

func execList(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	srcs := rc.Config.Sources

	if len(args) == 1 {
		if err := srcs.SetActiveGroup(args[0]); err != nil {
			return err
		}
	}

	if cmdFlagTrue(cmd, flagListGroups) {
		if len(args) > 0 {
			return errz.New("invalid: the --group / -g flag does not take arguments")
		}
		// Print groups instead of sources.
		//
		//  $ sq ls -g

		tree, err := srcs.Tree(source.RootGroup)
		if err != nil {
			return err
		}

		return rc.writers.srcw.Groups(tree)
	}

	return rc.writers.srcw.SourceSet(srcs)
}
