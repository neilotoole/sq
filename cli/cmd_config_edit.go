package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
)

func newConfigEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "edit",
		Args: cobra.NoArgs,
		ValidArgsFunction: (&handleTableCompleter{
			max: 1,
		}).complete,
		RunE:  execConfigEdit,
		Short: "Edit config",
		Long:  `Edit config in the editor specified by $EDITOR.`,
		Example: `  # Edit config
  $ sq config edit`,
	}

	return cmd
}

func execConfigEdit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	rc := RunContextFrom(ctx)
	_ = args
	_ = rc

	return errz.New("not implemented")
}
