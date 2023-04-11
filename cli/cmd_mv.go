package cli

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"golang.org/x/exp/slices"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "mv OLD NEW",
		Short:             "Move/rename sources and groups",
		Long:              "Move/rename sources and groups.",
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
  $ sq mv production prod
`,
	}

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
	rc := RunContextFrom(cmd.Context())
	_, err := rc.Config.Sources.RenameGroup(oldGroup, newGroup)
	if err != nil {
		return err
	}

	if err = rc.ConfigStore.Save(rc.Config); err != nil {
		return err
	}

	return rc.writers.srcw.Group(newGroup)
}

// execMoveHandleToGroup moves a source to a group.
//
//	$ sq mv @sakiladb prod
//	@prod/sakiladb
//
//	$ sq mv @prod/sakiladb /
//	@sakiladb
func execMoveHandleToGroup(cmd *cobra.Command, oldHandle, newGroup string) error {
	rc := RunContextFrom(cmd.Context())
	src, err := rc.Config.Sources.MoveHandleToGroup(oldHandle, newGroup)
	if err != nil {
		return err
	}

	if err = rc.ConfigStore.Save(rc.Config); err != nil {
		return err
	}

	return rc.writers.srcw.Source(src)
}

func execMoveRenameHandle(cmd *cobra.Command, oldHandle, newHandle string) error {
	rc := RunContextFrom(cmd.Context())
	src, err := rc.Config.Sources.RenameSource(oldHandle, newHandle)
	if err != nil {
		return err
	}

	if err = rc.ConfigStore.Save(rc.Config); err != nil {
		return err
	}

	return rc.writers.srcw.Source(src)
}

// completeMove is a completionFunc for the "mv" command.
func completeMove(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// srcs := RunContextFrom(cmd.Context()).Config.Sources

	// Example invocation:
	//
	//  sq mv @old_handle @new_handle
	//  sq mv
	//
	switch len(args) {
	case 0:
		// no args yet, so first
		c := completeHandle(2)
		return c(cmd, args, toComplete)

	case 1:
		switch {
		default:
			return nil, cobra.ShellCompDirectiveError
		case source.IsValidHandle(args[0]):
			// First argument is a valid handle.
			// The 2nd argument can be a handle or group.
			return completeMove2ndArg(cmd, args, toComplete)
		case source.IsValidGroup(args[0]):
			c := completeGroup(0)
			return c(cmd, args, toComplete)
		}

	// case 1:
	//	return []string{"."}, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	default:
		return nil, cobra.ShellCompDirectiveError
	}
}

func completeMove2ndArg(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if !source.IsValidHandle(args[0]) {
		return []string{}, cobra.ShellCompDirectiveError
	}

	var items []string
	groups, _ := completeGroup(0)(cmd, args, toComplete)
	slices.Sort(groups)

	switch {
	case toComplete == "":
		items = []string{"@"}
		items = append(items, groups...)
		return items, cobra.ShellCompDirectiveNoFileComp
	case toComplete == "/":
		// If toComplete is "/" (root), then it's a move to root.
		// We don't need any other suggestions. E.g.
		//
		//  $ sq mv @prod/db /
		//  @db
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	case toComplete[0] == '@':
		// Get rid of the "/" root group
		items = groups[1:]
		items = stringz.PrefixSlice(items, "@")
		items = stringz.SuffixSlice(items, "/")
		h := lastHandlePart(args[0])
		count := len(items)
		for i := 0; i < count; i++ {
			// Also offer the group plus the original name.
			items = append(items, items[i]+h)
		}

		items = lo.Without(items, args[0])

		return items, cobra.ShellCompDirectiveNoFileComp
	default:
	}

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
