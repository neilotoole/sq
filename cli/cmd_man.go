package cli

import (
	"fmt"
	"os"

	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/spf13/cobra"
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

func execGenerateMan(cmd *cobra.Command, args []string) error {
	manPage, err := mcobra.NewManPage(1, cmd.Root())
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(os.Stdout, manPage.Build(roff.NewDocument()))
	return err
}
