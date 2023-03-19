//go:build mage

// Magefile for building/test sq. This magefile was originally copied from
// the Hugo magefile, and may contain functionality that can be ditched.
package main

// See https://magefile.org
// This file originally derived from the Hugo magefile, see https://github.com/gohugoio/hugo

import (
	"fmt"
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

// Fmt runs gofumpt on the source.
func Fmt() error {
	return sh.RunV("gofumpt", "-l", "-w", ".")
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

func gitHeadCommit(short bool) string {
	args := []string{"rev-parse", "HEAD"}
	if short {
		args = []string{"rev-parse", "--short", "HEAD"}
	}

	hash, err := sh.Output("git", args...)
	panicIf(err)
	return hash
}

func gitCommitForLatestTag() string {
	hash, err := sh.Output("git", "rev-list", "--tags", "--max-count=1")
	panicIf(err)
	return hash
}

func gitTagForCommit(commit string) string {
	tag, err := sh.Output("git", "describe", "--tags", commit)
	panicIf(err)
	return tag
}

func gitLatestTag() string {
	commitForLatestTag := gitCommitForLatestTag()
	tag := gitTagForCommit(commitForLatestTag)
	return tag
}

func gitCurrentBranch() string {
	branch, err := sh.Output("git", "rev-parse", "--abbrev-ref", "HEAD")
	panicIf(err)
	return branch
}

func gitIsDirtyWorkingDir() bool {
	diff, err := sh.Output("git", "diff", "HEAD")
	panicIf(err)

	// If no diff, then diff's output will be empty.
	return diff != ""
}

// BuildVersion prints the build version that would be
// incorporated into the sq binary. The build version is of
// the form TAG[-SUFFIX], for example "v0.5.9-dev".
//   - If working dir is dirty, or if the HEAD commit does not
//     match the latest tag commit, the suffix is "-wip" (Work In
//     Progress), e.g. "v0.5.9-wip".
//   - Else, if the branch is not master, the branch name is
//     used as the suffix, e.g. "v0.5.9-dev".
//   - Else, we're on master and the HEAD commit is the latest
//     tag, so the suffix is omitted, e.g. "v0.5.9".
func BuildVersion() {
	fmt.Println(generateBuildVersion())
}

func generateBuildVersion() string {
	commitForLatestTag := gitCommitForLatestTag()
	headCommit := gitHeadCommit(false)
	latestTag := gitTagForCommit(commitForLatestTag)
	currentBranch := gitCurrentBranch()

	// If working dis is dirty or we're not on the latest tag,
	// then add a "-wip" suffix (Work In Progress).
	isDirty := gitIsDirtyWorkingDir()
	if isDirty || headCommit != commitForLatestTag {
		return latestTag + "-wip"
	}

	// Else, if the current branch is not master, append the
	// branch.
	if currentBranch != "master" {
		return latestTag + "-" + currentBranch
	}

	return latestTag
}
