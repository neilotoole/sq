package cli

import (
	"github.com/neilotoole/sq/cli/editor"
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

	ed := editor.NewDefaultEditor(editorEnvs)
	fp := "/Users/neilotoole/work/sq/sq/scratch/config-test.yml"
	if err := ed.Launch(fp); err != nil {
		return errz.Wrapf(err, "edit %s", fp)
	}

	return nil
}

var editorEnvs = []string{"SQ_EDITOR", "EDITOR"}
