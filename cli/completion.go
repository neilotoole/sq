package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/source"
)

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

	if toComplete == "" {
		rc.Log.Debug("toComplete is empty")
		return handles, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}

	if toComplete[0] == '@' {
		// We're dealing with handles
		if strings.ContainsRune(toComplete, '.') {
			// It's a handle with at least a partial table,
			// such as "@sakila_sl3.fil"
			handle, partialTbl, err := source.ParseTableHandle(toComplete)
			if err != nil {
				rc.Log.Error(err)
				return nil, cobra.ShellCompDirectiveError
			}

			tables, err := getTableNames(cmd.Context(), rc, handle)
			if err != nil {
				rc.Log.Error(err)
				return nil, cobra.ShellCompDirectiveError
			}

			var suggestions []string
			for _, table := range tables {
				if strings.HasPrefix(table, partialTbl) {
					suggestions = append(suggestions, handle+"."+table)
				}
			}

			return suggestions, cobra.ShellCompDirectiveNoFileComp
		}

		// Else, we've just got a handle
		var validHandles []string
		for _, handle := range handles {
			if strings.HasPrefix(handle, toComplete) {
				validHandles = append(validHandles, handle)
				//valid = append(valid, handles[i]+".")
			}
		}

		switch len(validHandles) {
		case 0:
			return nil, cobra.ShellCompDirectiveNoFileComp
		default:
			return validHandles, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
		case 1:
		}

		tables, err := getTableNames(cmd.Context(), rc, validHandles[0])
		if err != nil {
			rc.Log.Error(err)
			return nil, cobra.ShellCompDirectiveError
		}

		suggestions := []string{validHandles[0]}
		for _, table := range tables {
			suggestions = append(suggestions, validHandles[0]+"."+table)
		}

		return suggestions, cobra.ShellCompDirectiveNoFileComp

	}

	rc.Log.Warn("don't yet support table name completion")
	return nil, cobra.ShellCompDirectiveError
}

//rc.Log.Debug("toComplete is: ", toComplete)
//validHandles := make([]string, 0, len(handles))
//for i := range handles {
//	if strings.HasPrefix(handles[i], toComplete) {
//		validHandles = append(validHandles, handles[i])
//		//valid = append(valid, handles[i]+".")
//	}
//}
//}

//if len(validHandles) == 1 {
//	// Need to manually invoke preRunE to late init rc,
//	// because the completion functions are outside of the typical
//	// command sequence.
//	err := rc.preRunE()
//	if err != nil {
//		rc.Log.Error(err)
//		return nil, cobra.ShellCompDirectiveError
//	}
//
//	src, err := rc.Config.Sources.Get(validHandles[0])
//	if err != nil {
//		rc.Log.Error(err)
//		return nil, cobra.ShellCompDirectiveError
//	}
//
//	db, err := rc.databases.Open(cmd.Context(), src)
//	if err != nil {
//		rc.Log.Error(err)
//		return nil, cobra.ShellCompDirectiveError
//	}
//
//	md, err := db.SourceMetadata(cmd.Context())
//	if err != nil {
//		rc.Log.Error(err)
//		return nil, cobra.ShellCompDirectiveError
//	}
//
//	suggestions := []string{validHandles[0]}
//	tables := md.TableNames()
//	for _, table := range tables {
//		suggestions = append(suggestions, validHandles[0]+"."+table)
//	}
//
//	return suggestions, cobra.ShellCompDirectiveNoFileComp
//
//	//validHandles = append(validHandles, validHandles[0]+".")
//	//validHandles = append(validHandles, validHandles[0]+" ")
//}
//
////return nil, cobra.ShellCompDirectiveError
//rc.Log.Debug("Returning valid: ", validHandles)
//return validHandles, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
//}

func getTableNames(ctx context.Context, rc *RunContext, handle string) ([]string, error) {
	// May need to manually invoke preRunE to late init rc,
	// because the completion functions are outside of the typical
	// command sequence.
	if rc.databases == nil {
		err := rc.preRunE()
		if err != nil {
			return nil, err
		}
	}

	src, err := rc.Config.Sources.Get(handle)
	if err != nil {
		return nil, err
	}

	db, err := rc.databases.Open(ctx, src)
	if err != nil {
		return nil, err
	}

	md, err := db.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	return md.TableNames(), nil
}
