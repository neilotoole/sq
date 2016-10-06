// Package bootstrap is intended such that its init() method runs early in the
// program execution, so that it can initialize critical infrastructure such
// as logging and config, as these (logging in particular) can be used by the
// other packages' init functions. Someday I'll untangle all this dependency graph.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

func init() {

	// The location of the log file can be specified via an envar.
	path, ok := os.LookupEnv("SQ_LOGFILE")
	if !ok {
		// If the envar does not exist, we set it ourselves.
		path = filepath.Join(configDir(), "sq.log")
	}

	os.Setenv("__LG_LOG_FILEPATH", path)

	// The location of the config file can be specified via an envar.
	_, ok = os.LookupEnv("SQ_CONFIG_FILEPATH")
	if !ok {
		// If the envar does not exist, we set it ourselves.
		path := filepath.Join(configDir(), "sq.yml")
		os.Setenv("SQ_CONFIG_FILEPATH", path)
	}
}

var cfgDir string

// configDir returns the absolute path of "~/.sq/".
func configDir() string {

	if cfgDir != "" {

		return cfgDir
	}

	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to get user homedir: %v", err)
		os.Exit(1)
	}

	cfgDir = filepath.Join(home, ".sq")

	return cfgDir
}
