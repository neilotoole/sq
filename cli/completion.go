package cli

import "github.com/spf13/cobra"

// completionFunc is a shell completion function for a flag.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func completeHandles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	//rc, err := newLightRunContext()
	//if err != nil {
	//	return nil, cobra.ShellCompDirectiveError
	//}
	//
	//vals, err := fn(rc)
	//if err != nil {
	//	return []string{}, cobra.ShellCompDirectiveError
	//}

	vals := []string{"@my1", "@pg1"}

	return vals, cobra.ShellCompDirectiveNoFileComp
}
