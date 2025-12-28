package tu

import (
	"fmt"
	"runtime"
	"testing"
)

// SkipShort skips tb if testing.Short and arg skip are both true.
func SkipShort(tb testing.TB, skip bool) {
	tb.Helper()
	if skip && testing.Short() {
		tb.Skip("Skip long-running test because -short is true.")
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
	GH355SQLiteDecimalWin   GHIssue = 355 // https://github.com/neilotoole/sq/issues/355
	GH371ExcelSlowWin       GHIssue = 371 // https://github.com/neilotoole/sq/issues/371
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
