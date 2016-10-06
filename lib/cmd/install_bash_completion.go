package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:    "install-bash-completion",
		Short:  "Install bash completion script to /usr/local/etc/bash_completion.d",
		Long:   `Note: Mac OS X only`,
		RunE:   installBashCompletion,
		Hidden: true,
	}

	preprocessCmd(cmd)
	RootCmd.AddCommand(cmd)
}

func installBashCompletion(cmd *cobra.Command, args []string) error {

	doInstallBashCompletion()
	return nil
}

func doInstallBashCompletion() {
	path := "/usr/local/etc/bash_completion.d/sq"

	// TODO: only write if necessary (check for version/timestamp)
	err := RootCmd.GenBashCompletionFile(path) // TODO: install bash completion automatically
	if err != nil {
		log.Println(err)
	}

	//_, err = exec.Command("/bin/bash", "source", path).Output()
	//if err != nil {
	//	log.Println(err)
	//}

	//fmt.Printf("Bash completion script written: %v\nYou may need to restart your shell.\n", path)

	//log.Println(err)
}
