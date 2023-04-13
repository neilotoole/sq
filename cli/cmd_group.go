package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "group [GROUP]",
		RunE:              execGroup,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeGroup(1),
		Short:             "Get or set active group",
		Long: `Get or set active group. If no argument provided, get the active group.
Otherwise, set GROUP as the active group. An error is returned if GROUP does
not exist.

Use 'sq ls -g' to list groups.`,
		Example: `  # Get active group ("dev" in this case).
  $ sq group
  dev

  # Set "prod" as active group
  $ sq group prod
  prod

  # Reset to the root group
  $ sq group /
  /`,
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)

	return cmd
}

func execGroup(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	if len(args) == 0 {
		// Get the active group
		group := cfg.Sources.ActiveGroup()
		return rc.writers.srcw.Group(group)
	}

	group := strings.TrimSpace(args[0])

	if group == "/" {
		group = ""
	}
	if err := cfg.Sources.SetActiveGroup(group); err != nil {
		return err
	}

	if err := rc.ConfigStore.Save(cfg); err != nil {
		return err
	}

	return rc.writers.srcw.SetActiveGroup(group)
}

// newGroupListCommand is the hidden "sq lsg" command. It is
// a convenience for "sq ls --groups --verbose".
//
// But, it's a real bad UX smell. It is hidden because of shame.
// At one point, I had implemented this as "sq groups", but that
// also didn't feel right. The "sq ls -g" command seems like the right
// thing.
//
// When I have some more experience using "sq ls -gv" vs "sq lsg",
// hopefully this will get erased from the history books.
func newGroupListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "lsg",
		Hidden:  true,
		RunE:    execGroupList,
		Args:    cobra.NoArgs,
		Short:   "List groups",
		Long:    `List groups.`,
		Example: `  $ sq lsg`,
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	return cmd
}

func execGroupList(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	rc.writers.fm.Verbose = true
	return rc.writers.srcw.Groups(cfg.Sources.ActiveGroup(), cfg.Sources.Groups())
}
