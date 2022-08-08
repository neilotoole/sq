//go:build mage

// This magefile contains release targets and related functions.
// Ultimately this should go away in favor of CI.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Release is a mage namespace for release targets.
type Release mg.Namespace

// Snapshot runs goreleaser on Docker in snapshot mode.
func (Release) Snapshot() error {
	mg.Deps(CheckDocker)
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = execDocker("run",
		"--rm", "--privileged",
		"-v", wd+":/go/src/github.com/neilotoole/sq",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-w", "/go/src/github.com/neilotoole/sq",
		"neilotoole/xcgo:latest",
		// actual args to goreleaser are next
		"goreleaser", "--rm-dist", "--debug", "--snapshot")
	return err
}

// Release executes goreleaser on Docker. It will publish artifacts
// to github, brew, scoop, etc.
//
// To create a new tag, in this example: v0.5.1
//
//	$ git tag v0.5.1 && git push origin v0.5.1
//
// To delete a tag:
//
//	$ git push --delete origin v0.5.1 && git tag -d v0.5.1
func (r Release) Release() error {
	mg.Deps(r.GitIsReady, CheckDocker)
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	token, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok || token == "" {
		return errors.New("envar GITHUB_TOKEN is not set or is empty")
	}

	args := []string{
		"--rm", "--privileged",
		"-e", "GITHUB_TOKEN=" + token,
		"-v", wd + ":/go/src/github.com/neilotoole/sq",
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-w", "/go/src/github.com/neilotoole/sq",
		"neilotoole/xcgo:latest",
		"goreleaser", "--rm-dist",
	}

	snapLoginFile, ok := os.LookupEnv("SNAPCRAFT_LOGIN_FILE")
	if ok {
		localPath, err := filepath.Abs(snapLoginFile)
		if err != nil {
			return err
		}
		_, err = os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("failed to stat $SNAPCRAFT_LOGIN_FILE: %w", err)
		}

		args = append([]string{
			"-e", "SNAPCRAFT_LOGIN_FILE=/.snapcraft.login",
			"-v", localPath + ":/.snapcraft.login"},
			args...)
	}

	err = execDocker("run", args...)

	return err
}

// GitIsReady returns an error if the working dir is dirty or
// if the commit for the latest tag is not the HEAD commit.
func (Release) GitIsReady() error {
	if gitIsDirtyWorkingDir() {
		return errors.New("working dir is dirty")
	}

	if gitCommitForLatestTag() != gitHeadCommit(false) {
		return errors.New("the HEAD commit is not commit for latest tag")
	}

	fmt.Println("git is clean")
	return nil
}

// GitInfo prints some git info.
func (Release) GitInfo() {
	fmt.Println("gitCurrentBranch:", gitCurrentBranch())
	fmt.Println("gitHeadCommit(long):", gitHeadCommit(false))
	fmt.Println("gitHeadCommit(short):", gitHeadCommit(true))
	fmt.Println("getLatestTag:", gitLatestTag())
	commitForLatestTag := gitCommitForLatestTag()
	fmt.Println("gitCommitForLatestTag:", commitForLatestTag)
	tag := gitTagForCommit(commitForLatestTag)
	fmt.Println("gitTagForCommit:", tag)
	fmt.Println("generateBuildVersion:", generateBuildVersion())
	fmt.Println("gitIsDirtyWorkingDir:", gitIsDirtyWorkingDir())
	if gitCommitForLatestTag() != gitHeadCommit(false) {
		fmt.Println("\n*** HEAD commit is not commit for latest tag ***")
	}
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
func (Release) BuildVersion() {
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
