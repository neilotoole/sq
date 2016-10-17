package cmd

import (
	"fmt"

	"strings"

	"github.com/neilotoole/sq/cmd/assets"
	"github.com/neilotoole/sq/cmd/out"
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

	version, timestamp := getBuildInfo()

	// TODO: move to using getWriter() mechanism
	out.Color.Hilite.Printf("sq %s", version)
	fmt.Printf("                               ")
	out.Color.Faint.Printf(timestamp)
	fmt.Println()
	fmt.Printf("simple queryer for structured data      ")
	out.Color.Faint.Printf("http://neilotoole.io/sq")
	fmt.Println()

	return nil
}

func getBuildInfo() (version string, timestamp string) {

	bytes, err := assets.Asset("build_version.txt")
	if err != nil {
		panic(err)
	}
	version = strings.TrimSpace(string(bytes))

	bytes, err = assets.Asset("build_timestamp.txt")
	if err != nil {
		panic(err)
	}
	timestamp = strings.TrimSpace(string(bytes))
	return

}
