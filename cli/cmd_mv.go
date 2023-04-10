package cli

import (
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mv OLD NEW",
		Short: "Move/rename sources and groups",
		Long:  "Move/rename sources and groups.",
		Args:  cobra.ExactArgs(2),
		RunE:  execMove,
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
	case source.ValidHandle(args[0]) == nil && source.ValidHandle(args[1]) == nil:
		// It's a handle rename
		return execMoveRenameHandle(cmd, args[0], args[1])
	default:
		return errz.New("invalid args: see 'sq mv --help'")
	}
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
