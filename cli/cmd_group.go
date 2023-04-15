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
		groupName := cfg.Sources.ActiveGroup()
		tree, err := cfg.Sources.Tree(groupName)
		if err != nil {
			return err
		}

		return rc.writers.srcw.Group(tree)
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

	groupName := cfg.Sources.ActiveGroup()
	tree, err := cfg.Sources.Tree(groupName)
	if err != nil {
		return err
	}

	return rc.writers.srcw.Group(tree)
}
