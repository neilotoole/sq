package tu

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/neilotoole/sq/testh/proj"
)

// SkipShort skips tb if testing.Short and arg skip are both true.
func SkipShort(tb testing.TB, skip bool) {
	tb.Helper()
	if skip && testing.Short() {
		tb.Skip("Skip long-running test because -short is true.")
	}
}

// EnvNetwork is the envar that opts a test into using the real network.
const EnvNetwork = "SQ_TEST_NETWORK"

// SkipNoNetwork skips tb unless the EnvNetwork envar is set to a true value.
//
// Almost every fixture fetch is served by the local fixture server
// (testh/fixtsrv). The handful of tests that deliberately exercise a real
// remote host use this gate, so the CI dev loop, release tags and the DB
// matrix never depend on that host being reachable. main.yml sets EnvNetwork
// on the nightly schedule only. See gh #1158.
//
// tu.SkipShort is deliberately NOT used for this: main.yml passes -short only
// on the PR and master dev loop, so a -short gate would still leave the fetch
// running on release tags.
func SkipNoNetwork(tb testing.TB) {
	tb.Helper()
	if !proj.BoolEnvar(EnvNetwork) {
		tb.Skipf("Skip live-network test because %s is not true.", EnvNetwork)
	}
}

// GHIssue is a GitHub issue number. It is used
// with SkipIssue and SkipIssueWindows.
type GHIssue uint

// String returns the URL of the GitHub issue.
func (g GHIssue) String() string {
	return fmt.Sprintf("https://github.com/neilotoole/sq/issues/%d", g)
}

const (
	GH372ShellCompletionWin GHIssue = 372 // https://github.com/neilotoole/sq/issues/372
)

// SkipIssue skips tb due to the specified GitHub issue.
func SkipIssue(tb testing.TB, issue GHIssue) {
	tb.Helper()
	tb.Skipf("Skip because of GitHub issue #%d: %s", issue, issue)
}

const isWindows = runtime.GOOS == "windows"

// SkipIssueWindows skips tb on windows due to the specified GitHub issue.
func SkipIssueWindows(tb testing.TB, issue GHIssue) {
	tb.Helper()
	if !isWindows {
		return
	}

	tb.Skipf("Skip on Windows because of GitHub issue #%d: %s", issue, issue)
}

// SkipIf skips tb if cond is true. If msgAndArgs is non-empty, its first
// element must be a string, which can be a format string if there are
// additional elements.
//
// Examples:
//
//	tu.SkipIf(t, a == b)
//	tu.SkipIf(t, a == b, "skipping because a == b")
//	tu.SkipIf(t, a == b, "skipping because a is %v and b is %v", a, b)
func SkipIf(tb testing.TB, cond bool, format string, args ...any) {
	tb.Helper()
	if cond {
		if format == "" {
			tb.SkipNow()
		} else {
			tb.Skipf(format, args...)
		}
	}
}

// SkipWindows skips tb if running on Windows.
func SkipWindows(tb testing.TB, format string, args ...any) { //nolint:goprintffuncname
	tb.Helper()
	if isWindows {
		tb.Skipf(format, args...)
	}
}

// SkipWindowsIf skips tb if running on Windows and b is true.
func SkipWindowsIf(tb testing.TB, cond bool, format string, args ...any) {
	tb.Helper()
	if isWindows && cond {
		tb.Skipf(format, args...)
	}
}

// SkipReadOnlyFileUnenforceable skips tb when an on-disk read-only file
// (chmod 0444) can't be relied on to block a write open. That's the case
// on Windows (the 0444 bits don't map to POSIX write-deny semantics) and
// when running as root (file permission bits are bypassed). Tests that use
// a 0444 file as a regression guard (a read-write open must fail) call this
// so they don't silently pass where the guard is a no-op.
func SkipReadOnlyFileUnenforceable(tb testing.TB) {
	tb.Helper()
	if isWindows {
		tb.Skip("Skip: chmod 0444 doesn't enforce read-only the same way on Windows")
	}
	if os.Geteuid() == 0 {
		tb.Skip("Skip: root bypasses file permission bits, so chmod 0444 won't block a write open")
	}
}
