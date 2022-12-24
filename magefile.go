//go:build mage

// Magefile for building/test sq. This magefile was originally copied from
// the Hugo magefile, and may contain functionality that can be ditched.
package main

// See https://magefile.org
// This file originally derived from the Hugo magefile, see https://github.com/gohugoio/hugo

import (
	"os"
	"path/filepath"
	"time"

	"github.com/magefile/mage/sh"
)

const (
	packageName = "github.com/neilotoole/sq"
	ldflags     = "-X $PACKAGE/cli/buildinfo.Version=$BUILD_VERSION -X $PACKAGE/cli/buildinfo.Timestamp=$BUILD_TIMESTAMP -X $PACKAGE/cli/buildinfo.Commit=$BUILD_COMMIT"
)

var Default = Install

func ldflagsEnv() map[string]string {
	hash := gitHeadCommit(true)

	return map[string]string{
		"PACKAGE":         packageName,
		"BUILD_COMMIT":    hash,
		"BUILD_TIMESTAMP": time.Now().Format("2006-01-02T15:04:05Z0700"),
		"BUILD_VERSION":   generateBuildVersion(),
		"BUILD_BRANCH":    gitCurrentBranch(),
	}
}

// Clean cleans the dist dirs, and binaries.
func Clean() error {
	if err := sh.Run("rm", "-rf", "./dist*"); err != nil {
		return err
	}

	// Delete the sq binary that "go build" might produce
	if err := sh.Rm("./sq"); err != nil {
		return err
	}

	if gopath, ok := os.LookupEnv("GOPATH"); ok {
		if err := sh.Rm(filepath.Join(gopath, "bin", "sq")); err != nil {
			return err
		}
	}

	return nil
}

// Build builds sq.
func Build() error {
	return sh.RunWith(ldflagsEnv(), "go", "build", "-ldflags", ldflags, packageName)
}

// Install installs the sq binary.
func Install() error {
	return sh.RunWith(ldflagsEnv(), "go", "install", "-ldflags", ldflags, packageName)
}

// Test runs go test.
func Test() error {
	return sh.RunV("go", "test", "-v", "./...")
}

// Lint runs the golangci-lint linters.
// The golangci-lint binary must be installed:
//
//	$ brew install golangci-lint
//
// See .golangci.yml for configuration.
func Lint() error {
	return sh.RunV("golangci-lint", "run", "./...")
}

// Generate generates SLQ parser Go files from the
// antlr grammar. Note that the antlr generator tool is Java-based; you
// must have Java installed.
func Generate() error {
	return sh.Run("go", "generate", "./...")
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

// CheckDocker verifies that docker is running by executing echo
// on alpine.
func CheckDocker() error {
	return execDocker("run", "-it", "alpine", "echo", "docker is working")
}

// execDocker executes a docker command with args, returning
// any error. Example:
//
//	execDocker("run", "-it", "alpine", "echo", "hello world")
func execDocker(cmd string, args ...string) error {
	args = append([]string{cmd}, args...)
	return sh.RunV("docker", args...)
}
