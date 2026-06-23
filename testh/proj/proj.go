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
	"fmt"
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

	// EnvTestConfigFile, when set, overrides the path of the test source
	// config file the harness loads (default: the in-repo test.sq.yml).
	// The override file is loaded and its ${scheme:path} placeholders are
	// resolved exactly like the default file.
	EnvTestConfigFile = "SQ_TEST_CONFIG_FILE"
)

var projDir string

func init() { //nolint:gochecknoinits
	envar, ok := os.LookupEnv(EnvPassw)
	if !ok || envar == "" {
		err := os.Setenv(EnvPassw, DefaultPassw)
		if err != nil {
			panic(err)
		}
	}

	// Capture any externally-set SQ_ROOT before we overwrite it below, so we
	// can warn that it is being ignored.
	externalRoot, _ := os.LookupEnv(EnvRoot)

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	path, ok := findProjDir(cwd)
	if !ok {
		panic("unable to determine sq project dir from cwd: " + cwd)
	}

	if msg, warn := externalRootWarning(externalRoot, path); warn {
		fmt.Fprintln(os.Stderr, msg)
	}

	// SQ_ROOT is synthetic: it is always derived in-process from cwd, never
	// read from the caller's environment. Any externally-set SQ_ROOT is
	// intentionally ignored, so a stale value exported for a sibling worktree
	// cannot hijack this run. We set the envar here so os.ExpandEnv (and the
	// ${env:SQ_ROOT} secret resolver) can read the derived value.
	if err = os.Setenv(EnvRoot, path); err != nil {
		panic(err)
	}

	projDir = path
}

// findProjDir walks up from startDir to locate the sq project root: the
// nearest ancestor directory (inclusive) whose go.mod declares the sq module.
// It returns the project root and true, or ("", false) if no ancestor is the
// project root (e.g. startDir is outside any sq checkout).
func findProjDir(startDir string) (dir string, ok bool) {
	dir = startDir
	for {
		if isProjDir(dir) {
			return dir, true
		}

		if os.IsPathSeparator(dir[len(dir)-1]) {
			// per filepath.Dir contract, we're at the root
			return "", false
		}

		dir = filepath.Dir(dir)
	}
}

// externalRootWarning returns a warning message and true when an external
// SQ_ROOT envar is set. The test harness ignores SQ_ROOT: the project root is
// always derived in-process from the working directory, so a lingering value
// (e.g. exported for a sibling worktree) silently does nothing and can confuse
// a later run. The message advises unsetting it and names derived, the root
// actually in use, so the developer can see the effective value. It returns
// ("", false) when external is unset or whitespace-only.
func externalRootWarning(external, derived string) (msg string, warn bool) {
	if strings.TrimSpace(external) == "" {
		return "", false
	}

	return fmt.Sprintf(
		"[sq/testh] warning: SQ_ROOT=%s is set but ignored; the test harness "+
			"derives the project root from the working directory (%s). Unset "+
			"SQ_ROOT to avoid confusion, especially across git worktrees.",
		external, derived), true
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
	defer func() {
		if cerr := f.Close(); cerr != nil {
			panic(cerr)
		}
	}()

	gotMatch = false // reuse var
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == `module github.com/neilotoole/sq` {
			gotMatch = true
			break
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	return gotMatch
}

// Dir returns the absolute path to the root of the sq project,
// determined in-process by walking up from the working directory at
// package-init time. An externally-set SQ_ROOT is ignored.
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
