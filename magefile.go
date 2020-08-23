// +build mage

// Magefile for building/test sq. This magefile was originally copied from
// the Hugo magefile, and may contain functionality that can be ditched.
package main

// See https://magefile.org
// This file originally derived from the Hugo magefile, see https://github.com/gohugoio/hugo

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	packageName = "github.com/neilotoole/sq"
)

var ldflags = "-X $PACKAGE/cli/buildinfo.Version=$BUILD_VERSION -X $PACKAGE/cli/buildinfo.Timestamp=$BUILD_TIMESTAMP -X $PACKAGE/cli/buildinfo.Commit=$BUILD_COMMIT"

// allow user to override go executable by running as GO=xxx mage ... on unix-like systems
var gocmd = "go"

func init() {
	if exe := os.Getenv("GO"); exe != "" {
		gocmd = exe
	}

	// We want to use Go 1.11 modules even if the source lives inside GOPATH.
	// The default is "auto".
	os.Setenv("GO111MODULE", "on")
}

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

// Clean cleans the dist dir, build dirs and sq binaries.
// We use `which` to iteratively find any binaries on the path
// named "sq" and delete them.
func Clean() error {
	logIf(rmAll("./dist/*"))
	logIf(rmAll("./grammar/build"))

	// Delete the sq binary that "go build" might produce
	logIf(sh.Rm("./sq"))

	for {
		// Find any other sq binaries
		sqPath, err := sh.Output("which", "sq")
		if err != nil || sqPath == "" {
			break
		}

		// Make sure it's actually our sq binary. We'll do this
		// by checking the version output.
		version, err := sh.Output(sqPath, "version")
		if err != nil || !strings.HasPrefix(version, "sq v") {
			break
		}

		logIf(sh.Rm(sqPath))
	}

	return nil
}

// Build builds sq.
func Build() error {
	mg.Deps(Vet)
	return sh.RunWith(ldflagsEnv(), gocmd, "build", "-ldflags", ldflags, "-tags", buildTags(), packageName)
}

// BuildRace builds sq with race detector enabled
func BuildRace() error {
	mg.Deps(Vet)
	return sh.RunWith(ldflagsEnv(), gocmd, "build", "-race", "-ldflags", ldflags, "-tags", buildTags(), packageName)
}

// Install installs the sq binary.
func Install() error {
	mg.Deps(Vet)
	return sh.RunWith(ldflagsEnv(), gocmd, "install", "-ldflags", ldflags, "-tags", buildTags(), packageName)
}

// testGoFlags returns -test.short if isCI returns false.
func testGoFlags() string {
	if isCI() {
		return ""
	}

	return "-test.short"
}

// Test runs go test.
func Test() error {
	mg.Deps(Vet)
	env := map[string]string{"GOFLAGS": testGoFlags()}
	return runCmd(env, gocmd, "test", "./...", "-tags", buildTags())
}

// TestAll runs go test including the "heavy" build tag.
func TestAll() error {
	mg.Deps(Vet)
	env := map[string]string{"GOFLAGS": testGoFlags()}
	return runCmd(env, gocmd, "test", "./...", "-tags", "heavy")
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

// Vet runs go vet.
func Vet() error {
	if err := sh.Run(gocmd, "vet", "./..."); err != nil {
		fmt.Println("WARNING: issue reported by go vet:", err)

		// return fmt.Errorf("error running go vet: %v", err)
	}
	return nil
}

// GenerateParser generates SLQ parser Go files from the
// antlr grammar. Note that the antlr generator tool is java-based.
func GenerateParser() error {
	// https://www.antlr.org/download/antlr-4.8-complete.jar

	err := ensureJava()
	if err != nil {
		log.Println("java is required by antlr to generate the Go parser files, but java may not be available:", err)
		return err
	}

	jarPath, err := ensureAntlrJar()
	if err != nil {
		return err
	}

	genDir, err := execAntlrGenerate(jarPath)
	if err != nil {
		return err
	}

	log.Println("Antlr generated files to:", genDir)
	err = updateParserFiles(genDir)
	if err != nil {
		return err
	}

	// Finally clean up the generated files
	return rmAll(filepath.Join(genDir, "*"))
}

// updateParserFiles updates/copies files from genDir to
// the version-controlled slq dir.
func updateParserFiles(genDir string) error {
	log.Println("Building generated parser files:", genDir)
	err := sh.RunV("go", "build", "-v", genDir)
	if err != nil {
		return err
	}

	log.Println("Deleting previous generated files from libsq/ast/internal/slq")

	err = rmAll("libsq/ast/internal/slq/*.go", "libsq/ast/internal/slq/*.token", "libsq/ast/internal/slq/*.interp")
	if err != nil {
		return err
	}

	log.Println("Copying generated files to libsq/ast/internal/slq")
	err = copyFilesToDir("libsq/ast/internal/slq", filepath.Join(genDir, "*"))
	if err != nil {
		return err
	}

	return nil
}

// execAntlrGenerate executes the antlr tool from jarPath,
// generates Go parser files into genDir.
func execAntlrGenerate(jarPath string) (genDir string, err error) {
	log.Println("Using antlr jar:", jarPath)

	// The antlr tool is finicky about paths, so we're going
	// to bypass this problem by using absolute paths.
	grammarFile := filepath.Join("grammar", "SLQ.g4")
	grammarFile, err = filepath.Abs(grammarFile)
	if err != nil {
		return "", err
	}

	genDir = filepath.Join("grammar", "build", "slq")
	genDir, err = filepath.Abs(genDir)
	if err != nil {
		return "", err
	}

	// Make sure the output dir exists.
	err = os.MkdirAll(genDir, 0700)
	if err != nil {
		return "", err
	}

	// Delete any existing files in output dir.
	err = rmAll(filepath.Join(genDir, "*"))
	if err != nil {
		return "", err
	}

	err = sh.Run("java",
		"-cp", jarPath, "org.antlr.v4.Tool",
		"-listener", "-visitor",
		"-package", "slq",
		"-Dlanguage=Go",
		"-o", genDir,
		grammarFile,
	)

	if err != nil {
		return "", err
	}

	return genDir, nil
}

// ensureJava ensures that java is available.
func ensureJava() error {
	err := sh.Run("java", "-version")
	if err != nil {
		log.Println("Didn't find java executable")
		return err
	}

	log.Println("Found java executable")
	return nil
}

// ensureAntlrJar ensures that we have the antlr jar
// available, and returns the absolute path to the jar.
// The antlr jar can be found in grammar/build.
func ensureAntlrJar() (jarPath string, err error) {
	const (
		baseDownloadURL = "https://www.antlr.org/download/"
		jarName         = "antlr-4.7.2-complete.jar"
	)

	// Make sure our grammar/build dir exists
	err = os.MkdirAll(filepath.Join("grammar", "build"), 0700)
	if err != nil {
		return "", err
	}

	path := filepath.Join("grammar", "build", jarName)
	path, err = filepath.Abs(path)
	panicIf(err)

	_, err = os.Stat(path)
	if err == nil {
		log.Println("Found antlr jar:", path)
		return path, nil
	}

	log.Println("Did not find antlr jar at:", path)

	// Create the file
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	downloadURL := baseDownloadURL + jarName
	log.Println("Downloading jar from:", downloadURL)

	// Get the data
	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}

	log.Printf("Downloaded %d bytes to %s", resp.ContentLength, path)
	return path, nil
}

func runCmd(env map[string]string, cmd string, args ...string) error {
	if mg.Verbose() {
		return sh.RunWith(env, cmd, args...)
	}
	output, err := sh.OutputWith(env, cmd, args...)
	if err != nil {
		fmt.Fprint(os.Stderr, output)
	}

	return err
}

func isGoLatest() bool {
	return strings.Contains(runtime.Version(), "1.14")
}

func isCI() bool {
	return os.Getenv("CI") != ""
}

func buildTags() string {
	if envtags := os.Getenv("BUILD_TAGS"); envtags != "" {
		return envtags
	}
	return "none"
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func logIf(err error) {
	if err != nil {
		log.Println(err)
	}
}

// rmAll deletes all files or dirs matching a pattern.
func rmAll(patterns ...string) error {
	var allMatches []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		allMatches = append(allMatches, matches...)
	}

	for _, match := range allMatches {
		log.Printf("rmAll: %s", match)
		err := sh.Rm(match)
		if err != nil {
			return err
		}
	}

	return nil
}

// copyFilesToDir copies all files matching a pattern to dstDir.
// Because of flattening that occurs, if multiple files have the
// same name in dstDir, the later file will be be the last one
// copied and thus take effect. Directories are skipped.
func copyFilesToDir(dstDir string, patterns ...string) error {
	var allMatches []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}

		for _, fname := range matches {
			fi, err := os.Stat(fname)
			if err != nil {
				log.Printf("copyFilesToDir: skipping %s because: %v", fname, err)
				continue
			}
			if fi.IsDir() {
				log.Printf("copyFilesToDir: skipping %s because it's a dir", fname)
				continue
			}

			allMatches = append(allMatches, fname)
		}
	}

	for _, match := range allMatches {
		_, name := filepath.Split(match)
		dstFile := filepath.Join(dstDir, name)
		log.Printf("copyFilesToDir: %s --> %s\n", match, dstFile)

		err := sh.Copy(dstFile, match)
		if err != nil {
			return err
		}
	}

	return nil
}

// CheckDocker verifies that docker is running by executing echo
// on alpine.
func CheckDocker() error {
	return execDocker("run", "-it", "alpine", "echo", "docker is working")
}

// execDocker executes a docker command with args, returning
// any error. Example:
//
//  execDocker("run", "-it", "alpine", "echo", "hello world")
func execDocker(cmd string, args ...string) error {
	args = append([]string{cmd}, args...)
	return sh.RunV("docker", args...)
}
