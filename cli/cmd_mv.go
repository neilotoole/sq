package cli

import (
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
)

func newMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mv OLD NEW",
		Short: "Move/rename sources and groups",
		Long: `Move/rename sources and groups.

The mv command works analogously to the UNIX "mv" command, where
source handles are files, and groups are directories.`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeMove,
		RunE:              execMove,
		Example: `  # Rename a source
  $ sq mv @sakila_postgres @sakila_pg
  @sakila_pg

  # Move a source into a group (same as renaming)
  $ sq mv @sakila_pg @sakila/pg
  @sakila/pg

  # Move a source into a group, keep current name
  $ sq mv @sakila/pg prod
  @prod/pg

  # Rename the group "production" to "prod". This will rename
  # any sources already in that group.
  $ sq mv production prod`,
	}

	markCmdRequiresConfigLock(cmd)
	addTextFormatFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	return cmd
}

func execMove(cmd *cobra.Command, args []string) error {
	switch {
	case source.IsValidHandle(args[0]) && source.IsValidHandle(args[1]):
		// Effectively a handle rename
		// sq mv @staging/db @prod/db
		return execMoveRenameHandle(cmd, args[0], args[1])
	case source.IsValidHandle(args[0]) && source.IsValidGroup(args[1]):
		// sq mv @staging/db prod
		return execMoveHandleToGroup(cmd, args[0], args[1])
	case source.IsValidGroup(args[0]) && source.IsValidGroup(args[1]):
		return execMoveRenameGroup(cmd, args[0], args[1])
	default:
		return errz.New("invalid args: see 'sq mv --help'")
	}
}

// execMoveRenameGroup renames a group.
//
//	$ sq mv production prod
//	prod
func execMoveRenameGroup(cmd *cobra.Command, oldGroup, newGroup string) error {
	ru := run.FromContext(cmd.Context())
	_, err := ru.Config.Collection.RenameGroup(oldGroup, newGroup)
	if err != nil {
		return err
	}

	if _, err = source.VerifyIntegrity(ru.Config.Collection); err != nil {
		return err
	}

	if err = ru.ConfigStore.Save(cmd.Context(), ru.Config); err != nil {
		return err
	}

	tree, err := ru.Config.Collection.Tree(newGroup)
	if err != nil {
		return err
	}

	return ru.Writers.Source.Group(tree)
}

// execMoveHandleToGroup moves a source to a group.
//
//	$ sq mv @sakiladb prod
//	@prod/sakiladb
//
//	$ sq mv @prod/sakiladb /
//	@sakiladb
func execMoveHandleToGroup(cmd *cobra.Command, oldHandle, newGroup string) error {
	ru := run.FromContext(cmd.Context())
	oldSrc, err := ru.Config.Collection.Get(oldHandle)
	if err != nil {
		return err
	}

	// Make a clone, because the original may get modified.
	oldSrc = oldSrc.Clone()
	newSrc, err := ru.Config.Collection.MoveHandleToGroup(oldHandle, newGroup)
	if err != nil {
		return err
	}

	if _, err = source.VerifyIntegrity(ru.Config.Collection); err != nil {
		return err
	}

	if err = ru.ConfigStore.Save(cmd.Context(), ru.Config); err != nil {
		return err
	}

	return ru.Writers.Source.Moved(ru.Config.Collection, oldSrc, newSrc)
}

// execMoveRenameHandle renames a handle.
//
//	$ sq mv @sakila_db @sakiladb
//	$ sq mv @sakiladb @sakila/db
func execMoveRenameHandle(cmd *cobra.Command, oldHandle, newHandle string) error {
	ru := run.FromContext(cmd.Context())
	oldSrc, err := ru.Config.Collection.Get(oldHandle)
	if err != nil {
		return err
	}

	// Make a clone, because the original may get modified.
	oldSrc = oldSrc.Clone()

	newSrc, err := ru.Config.Collection.RenameSource(oldHandle, newHandle)
	if err != nil {
		return err
	}

	if _, err = source.VerifyIntegrity(ru.Config.Collection); err != nil {
		return err
	}

	if err = ru.ConfigStore.Save(cmd.Context(), ru.Config); err != nil {
		return err
	}

	return ru.Writers.Source.Moved(ru.Config.Collection, oldSrc, newSrc)
}

// completeMove is a completionFunc for the "mv" command.
// Example invocations:
//
//	$ sq mv @old_handle @new_handle				# Rename handle
//	$ sq mv @prod/old_handle @dev/old_handle	# Rename handle in group
//	$ sq mv @prod/old_handle /					# Move handle to root group
//	$ sq mv @prod/old_handle dev				# Move handle to group
//	$ sq mv prod dev							# Rename group
func completeMove(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		// No args yet, so first arg could be a handle or group.
		return completeHandleOrGroup(cmd, args, toComplete)
	case 1:
		// Continue below.
	default:
		// Maximum two values (the 2nd arg is in toComplete), so it's an error.
		return nil, cobra.ShellCompDirectiveError
	}

	// We're processing the 2nd cmd arg.
	// Note that the 2nd cmd arg value is found in toComplete, not args[1].
	arg0 := args[0]

	if !source.IsValidHandle(arg0) && !source.IsValidGroup(arg0) {
		// arg0 is not valid.
		return nil, cobra.ShellCompDirectiveError
	}

	var items []string
	groups, _ := completeGroup(0)(cmd, args, toComplete)
	slices.Sort(groups)

	switch {
	case toComplete == "":
		switch {
		case source.IsValidHandle(arg0):
			// If arg0 is a handle, the 2nd arg can be either
			// a handle or group.
			items = []string{"@"}
			items = append(items, groups...)
			return items, cobra.ShellCompDirectiveNoFileComp

			// return completeHandleOrGroup(cmd, args, toComplete)
		case source.IsValidGroup(arg0):
			// If arg0 is a group, the 2nd arg can only be a group.
			return completeGroup(0)(cmd, args, toComplete)
		default:
			// Shouldn't be possible.
			return nil, cobra.ShellCompDirectiveError
		}
	case toComplete == "/":
		// If toComplete is "/" (root), then it's a move to root.
		//
		//  $ sq mv @prod/db /
		//  @db
		//
		//  $ sq mv prod /
		//  /
		//
		// No need to offer any other possibilities.
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	case toComplete[0] == '@':
		// If toComplete is a handle, then the arg0 must be a handle.
		if !source.IsValidHandle(arg0) {
			return nil, cobra.ShellCompDirectiveError
		}

		// Get rid of the "/" root group
		items = groups[1:]
		items = stringz.PrefixSlice(items, "@")
		items = stringz.SuffixSlice(items, "/")
		h := lastHandlePart(arg0)
		count := len(items)
		for i := 0; i < count; i++ {
			// Also offer the group plus the original name.
			items = append(items, items[i]+h)
		}

		items = lo.Without(items, args[0])
		items = lo.Reject(items, func(item string, index int) bool {
			return !strings.HasPrefix(item, toComplete)
		})
		return items, cobra.ShellCompDirectiveNoFileComp
	default:
		// toComplete must be a group. Continue below.
	}

	// toComplete must be a group.
	if !source.IsValidGroup(toComplete) {
		return nil, cobra.ShellCompDirectiveError
	}

	items = append(items, groups...)
	items = lo.Reject(items, func(item string, index int) bool {
		return !strings.HasPrefix(item, toComplete)
	})
	items, _ = lo.Difference(items, args)
	items = lo.Uniq(items)
	slices.Sort(items)

	return items, cobra.ShellCompDirectiveNoFileComp
}

// lastHandlePart returns the part of the handle after
// the final (if any slash), with the @ prefix trimmed.
// The h arg must be a valid handle.
func lastHandlePart(h string) string {
	h = strings.TrimPrefix(h, "@")
	i := strings.LastIndex(h, "/")
	if i != -1 {
		h = h[i+1:]
	}

	return h
}
