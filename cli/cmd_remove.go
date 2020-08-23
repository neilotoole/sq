package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
)

func newSrcRemoveCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:     "rm @HANDLE",
		Example: `  sq rm @my1`,
		Aliases: []string{"remove"},
		Short:   "Remove data source",
	}

	return cmd, execSrcRemove
}

func execSrcRemove(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errz.Errorf(msgInvalidArgs)
	}

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
	_, _ = rc.writers.fmt.Hilite.Fprintf(rc.Out, "%s", src.Handle)
	fmt.Fprintln(rc.Out)

	return nil
}
