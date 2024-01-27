package tu

import (
	"fmt"
	"runtime"
	"testing"
)

// SkipShort skips t if testing.Short and arg skip are both true.
func SkipShort(t testing.TB, skip bool) {
	if skip && testing.Short() {
		t.Skip("Skip long-running test because -short is true.")
	}
}

// GHIssue is a GitHub issue number.
type GHIssue uint

// String returns the URL of the GitHub issue.
func (g GHIssue) String() string {
	return fmt.Sprintf("https://github.com/neilotoole/sq/issues/%d", g)
}

const (
	GH355SQLiteDecimalWin GHIssue = 355 // https://github.com/neilotoole/sq/issues/355
)

// SkipIssue skips t due to the specified GitHub issue.
func SkipIssue(t testing.TB, issue GHIssue) {
	t.Skipf("Skip because of GitHub issue #%d: %s", issue, issue)
}

const isWindows = runtime.GOOS == "windows"

// SkipIssueWindows skips t on windows due to the specified GitHub issue.
func SkipIssueWindows(t testing.TB, issue GHIssue) {
	if !isWindows {
		return
	}

	t.Skipf("Skip on Windows because of GitHub issue #%d: %s", issue, issue)
}

// SkipIff skips t if b is true. If msgAndArgs is non-empty, its first
// element must be a string, which can be a format string if there are
// additional elements.
//
// Examples:
//
//	tu.SkipIff(t, a == b)
//	tu.SkipIff(t, a == b, "skipping because a == b")
//	tu.SkipIff(t, a == b, "skipping because a is %v and b is %v", a, b)
func SkipIff(t testing.TB, b bool, format string, args ...any) {
	if b {
		if format == "" {
			t.SkipNow()
		} else {
			t.Skipf(format, args...)
		}
	}
}

// SkipWindows skips t if running on Windows.
func SkipWindows(t testing.TB, format string, args ...any) {
	if isWindows {
		t.Skipf(format, args...)
	}
}

// SkipWindowsIf skips t if running on Windows and b is true.
func SkipWindowsIf(t testing.TB, b bool, format string, args ...any) {
	if isWindows && b {
		t.Skipf(format, args...)
	}
}
