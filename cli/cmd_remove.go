package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSrcRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "rm @HANDLE",
		Example:           `  $ sq rm @my1`,
		Short:             "Remove data source",
		Args:              cobra.ExactArgs(1),
		RunE:              execSrcRemove,
		ValidArgsFunction: completeHandle(1),
	}

	return cmd
}

func execSrcRemove(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config
	src, err := cfg.Sources.Get(args[0])
	if err != nil {
		return err
	}

	err = cfg.Sources.Remove(src.Handle)
	if err != nil {
		return err
	}

	err = rc.ConfigStore.Save(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(rc.Out, "Removed data source ")
	_, _ = rc.writers.fm.Hilite.Fprintf(rc.Out, "%s", src.Handle)
	fmt.Fprintln(rc.Out)

	return nil
}
