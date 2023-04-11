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
Otherwise, set GROUP as the active group.`,
		Example: `  # Get active group
  $ sq group

  # Set "prod" as active group
  $ sq group prod`,
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

func newGroupListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "groups",
		RunE:    execGroupList,
		Args:    cobra.NoArgs,
		Short:   "List groups",
		Long:    `List groups`,
		Example: `  $ sq groups`,
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)

	return cmd
}

func execGroupList(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config

	return rc.writers.srcw.Groups(cfg.Sources.ActiveGroup(), cfg.Sources.Groups())
}
