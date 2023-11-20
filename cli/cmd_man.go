package cli

import (
	"fmt"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
)

func newManCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "man",
		Short:                 "Generate man pages",
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		Args:                  cobra.NoArgs,
		RunE:                  execGenerateMan,
	}

	return cmd
}

func execGenerateMan(cmd *cobra.Command, _ []string) error {
	ru := run.FromContext(cmd.Context())
	manPage, err := mcobra.NewManPage(1, cmd.Root())
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(ru.Out, manPage.Build(roff.NewDocument()))
	return err
}
