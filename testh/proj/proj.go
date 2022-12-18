// Package proj contains test utilities for dealing with project
// paths and the like. The sq project dir is accessed via the
// Dir function. When proj's init function returns, certain envars
// such as SQ_ROOT are guaranteed to be set. Thus the following
// will work as expected:
//
//	p := proj.Expand("${SQ_ROOT}/go.mod")
package proj

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (
	// EnvRoot is the name of the envar holding the path of
	// the sq project root.
	EnvRoot = "SQ_ROOT"

	// EnvPassw is the name of the envar holding the standard
	// password used for testing.
	EnvPassw = "SQ_PASSW"

	// DefaultPassw is the default password used for testing.
	DefaultPassw = "p_ssW0rd"

	EnvLogFile = "SQ_LOGFILE"
)

var projDir string

func init() {
	envar, ok := os.LookupEnv(EnvPassw)
	if !ok || envar == "" {
		err := os.Setenv(EnvPassw, DefaultPassw)
		if err != nil {
			panic(err)
		}
	}

	var path string
	envar, ok = os.LookupEnv(EnvRoot)
	if !ok || envar == "" {
		dir, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		for {
			if isProjDir(dir) {
				path = dir
				break
			}

			if os.IsPathSeparator(dir[len(dir)-1]) {
				// per filepath.Dir contract, we're at the root
				break
			}

			dir = filepath.Dir(dir)
		}

		if path == "" {
			panic("unable to determine sq project dir")
		}

		// If we get here, we've found the proj dir.

		// Set the env so that os.ExpandEnv etc can use the envar
		err = os.Setenv(EnvRoot, path)
		if err != nil {
			panic(err)
		}
	} else {
		var err error
		path, err = filepath.Abs(envar)
		if err != nil {
			panic(err)
		}

		if !isProjDir(path) {
			panic("envar " + EnvRoot + " does not appear to be sq project dir: " + envar)
		}
	}

	projDir = path
}

// isProjDir returns true if dir contains a go.mod file
// containing sq's module declaration. Any io error panics.
func isProjDir(dir string) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var gotMatch bool
	for _, fi := range files {
		if fi.Name() == "go.mod" {
			gotMatch = true
			break
		}
	}

	if !gotMatch {
		return false
	}

	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		panic(err)
	}

	gotMatch = false // reuse var
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == `module github.com/neilotoole/sq` {
			gotMatch = true
			break
		}
	}

	err = f.Close()
	if err != nil {
		panic(err)
	}
	return gotMatch
}

// Dir returns the absolute path to the root of the sq project,
// as set in envar EnvRoot or determined programmatically.
func Dir() string {
	return projDir
}

// Rel returns the relative path from the current
// working dir to path, where path is relative to the project dir.
// This is useful for accessing common test fixtures across
// packages.
func Rel(path string) string {
	abs := filepath.Join(Dir(), path)

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	relPath, err := filepath.Rel(cwd, abs)
	if err != nil {
		panic(err)
	}

	return relPath
}

// Abs returns the absolute path of projRelPath,
// where projRelPath is relative to the project dir.
func Abs(projRelPath string) string {
	return filepath.Join(Dir(), projRelPath)
}

// ReadFile is a convenience function for reading
// a file under the proj dir. It's equivalent to
// os.ReadFile(proj.Abs(path)) but panics on any error.
func ReadFile(projRelPath string) []byte {
	p := Abs(projRelPath)
	d, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return d
}

// Expand wraps os.ExpandEnv. This function is preferred
// as it ensures that envar SQ_ROOT is set (via
// proj's init function).
func Expand(s string) string {
	return os.ExpandEnv(s)
}

// Passw returns the password used for testing.
func Passw() string {
	return os.Getenv(EnvPassw)
}

// LogFile returns the path to sq's log file, as
// specified by EnvLogFile. If none set, the empty
// string is returned.
func LogFile() string {
	return os.Getenv(EnvLogFile)
}

// BoolEnvar returns true if the environment variable e is set
// and its value is boolean true.
func BoolEnvar(envar string) bool {
	s, ok := os.LookupEnv(envar)
	if !ok {
		return false
	}

	b, _ := stringz.ParseBool(strings.TrimSpace(s))
	return b
}
