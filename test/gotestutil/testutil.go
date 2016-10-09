// Package gotestutil should be imported (as the first import, using _) by all
// sq test files so that logging  and config are directed to appropriate files
// instead of using the default files.
package gotestutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/neilotoole/go-lg/lg"
)

func init() {
	fmt.Println("gotestutil init()")

	cfgDir := configDir()
	path := filepath.Join(cfgDir, "sq.test.log")

	parent := filepath.Dir(path)
	err := os.MkdirAll(parent, os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("unable to create parent dir for test log file path %q: %v", path, err))
		return
	}

	logFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(fmt.Sprintf(" unable to initialize test log file %q: %v", path, err))
	}

	fmt.Printf("using test log file: %v\n", logFile.Name())

	lg.Use(logFile)

}

// configDir returns the absolute path of "~/.sq/".
func configDir() string {

	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to get user homedir: %v", err)
		os.Exit(1)
	}

	return filepath.Join(home, ".sq")
}
