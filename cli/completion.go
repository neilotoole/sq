package cli

import "github.com/spf13/cobra"

// completionFunc is a shell completion function.
type completionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

var (
	_ completionFunc = completeHandles
	_ completionFunc = completeHandleOrTable
)

func completeHandles(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())

	handles := rc.Config.Sources.Handles()

	return handles, cobra.ShellCompDirectiveNoFileComp
}

func completeHandleOrTable(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rc := RunContextFrom(cmd.Context())
	rc.Log.Debug(args, toComplete)

	handles := rc.Config.Sources.Handles()
	count := len(handles)

	handles2 := make([]string, count*2)
	copy(handles2, handles)
	for i := count; i < len(handles2); i++ {
		handles2[i] = handles[i-count] + "."
	}

	//for i := 0; i < len(handles); i++ {
	//	handles = append(handles, handles[i]+".")
	//}

	if toComplete == "" {
		rc.Log.Debug("toComplete is empty")
		return handles2, cobra.ShellCompDirectiveNoSpace
	}

	//return nil, cobra.ShellCompDirectiveError
	rc.Log.Debug("toComplete is: ", toComplete)
	return handles2, cobra.ShellCompDirectiveNoSpace
}
