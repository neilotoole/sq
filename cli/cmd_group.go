package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
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

	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	return cmd
}

func execGroup(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	cfg := ru.Config

	if len(args) == 0 {
		// Get the active group
		groupName := cfg.Collection.ActiveGroup()
		tree, err := cfg.Collection.Tree(groupName)
		if err != nil {
			return err
		}

		return ru.Writers.Source.Group(tree)
	}

	group := strings.TrimSpace(args[0])

	if group == "/" {
		group = ""
	}
	if err := cfg.Collection.SetActiveGroup(group); err != nil {
		return err
	}

	if err := ru.ConfigStore.Save(cmd.Context(), cfg); err != nil {
		return err
	}

	groupName := cfg.Collection.ActiveGroup()
	tree, err := cfg.Collection.Tree(groupName)
	if err != nil {
		return err
	}

	return ru.Writers.Source.Group(tree)
}
