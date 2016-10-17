package cmd

import (
	"fmt"

	"strconv"

	"github.com/spf13/cobra"
)

var testerCmd = &cobra.Command{
	Use:    "tester",
	Hidden: true,
	Short:  "[DEV] This cmd is here for executing code under debug",
	RunE:   execTester,
}

func init() {
	preprocessCmd(testerCmd)
	RootCmd.AddCommand(testerCmd)

}

func execTester(cmd *cobra.Command, args []string) error {
	fmt.Println("execTest")

	names := []string{"one", "fourteen", "nineteeneightyfour"}
	maxLen := 18

	tpl := "%" + strconv.Itoa(maxLen) + "s wubble\n"

	for _, name := range names {
		fmt.Printf(tpl, name)
	}

	return nil
}
