package cli

import (
	"github.com/neilotoole/sq/cli/flag"
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

  # List groups (all) instead of sources
  $ sq ls -g

  # Print verbose group details
  $ sq ls -gv

  # List subgroups in "prod" group
  $ sq ls -g prod`,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().Bool(flag.Pretty, true, flag.PrettyUsage)
	cmd.Flags().BoolP(flag.ListGroup, flag.ListGroupShort, false, flag.ListGroupUsage)

	return cmd
}

func execList(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	coll := rc.Config.Collection

	if cmdFlagTrue(cmd, flag.ListGroup) {
		// We're listing groups, not sources.

		var fromGroup string
		switch len(args) {
		case 0:
			fromGroup = source.RootGroup
		case 1:
			if err := source.ValidGroup(args[0]); err != nil {
				return errz.Wrapf(err, "invalid value for --%s", flag.ListGroup)
			}
			fromGroup = args[0]
		default:
			return errz.Errorf("invalid: --%s takes a max of 1 arg", flag.ListGroup)
		}

		tree, err := coll.Tree(fromGroup)
		if err != nil {
			return err
		}

		return rc.writers.srcw.Groups(tree)
	}

	// We're listing sources, not groups.

	if len(args) == 1 {
		// We want to list the sources in a group. To do this, we
		// (temporarily) set the active group, and then continue below.
		// $ sq ls prod
		if err := coll.SetActiveGroup(args[0]); err != nil {
			return err
		}
	}

	return rc.writers.srcw.Collection(coll)
}
