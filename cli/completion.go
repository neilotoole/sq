package cli

import "github.com/spf13/cobra"

// completionFunc is a shell completion function.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

var _ completionFunc = completeHandles

func completeHandles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())

	handles := rc.Config.Sources.Handles()

	return handles, cobra.ShellCompDirectiveNoFileComp
}
