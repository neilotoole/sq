package cmd

import (
	"runtime"

	"github.com/neilotoole/go-lg/lg"
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

	var path string

	switch runtime.GOOS {
	case "windows":
		lg.Debugf("skipping install bash completion on windows")
		return
	case "darwin":
		path = "/usr/local/etc/bash_completion.d/sq"
	default:
		// it's unixish
		path = " /etc/bash_completion.d/sq"
	}

	// TODO: only write if necessary (check for version/timestamp/checksum)
	err := RootCmd.GenBashCompletionFile(path)
	if err != nil {
		lg.Warnf("failed to write bash completion to %q: %v", path, err)
		return
	}

}
