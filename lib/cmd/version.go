package cmd

import (
	"fmt"

	"strings"

	"github.com/neilotoole/sq/lib/assets"
	"github.com/neilotoole/sq/lib/out"
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

	//color.Set(color.FgHiBlue)
	//fmt.Printf("sq %s", version)
	//color.Unset()
	//fmt.Printf("\n")

	out.Color.Hilite.Printf("sq %s", version)
	fmt.Printf("                           ")
	out.Color.Faint.Printf(timestamp)
	fmt.Println()
	fmt.Printf("simple queryer for structured data      ")
	out.Color.Faint.Printf("http://neilotoole.io/sq")
	fmt.Println()
	//fmt.Printf("(c) Neil O'Toole 2014-2016              neilotoole@apache.org\n")

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
