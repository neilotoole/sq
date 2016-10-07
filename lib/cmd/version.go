package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print sq version",
	RunE:  execVersion,
}

func init() {
	preprocessCmd(versionCmd)
	RootCmd.AddCommand(versionCmd)
}

func execVersion(cmd *cobra.Command, args []string) error {

	color.Set(color.FgHiBlue)
	fmt.Printf("sq 0.30.3")
	color.Unset()
	fmt.Printf("\n")
	fmt.Printf("Simple Queryer                http://neilotoole.io/sq\n")
	fmt.Printf("(c) Neil O'Toole 2014-2016    neilotoole@apache.org\n")

	return nil
}
