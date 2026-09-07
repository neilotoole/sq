package diffdoc

import "strconv"

// HunkRange formats one side of a unified-diff hunk header ("start,count"
// portion). It follows the unified diff specification:
//
//   - A count of exactly 1 is written as just the start line number (the
//     ",count" suffix is omitted): e.g. HunkRange(9, 1) → "9".
//   - A count of 0 is always written as "start,0" (used for pure insertions
//     or deletions where one side contributes no lines): e.g.
//     HunkRange(9, 0) → "9,0".
//   - Any other count is written as "start,count": e.g. HunkRange(9, 3) →
//     "9,3".
func HunkRange(start, count int) string {
	if count == 1 {
		return strconv.Itoa(start)
	}
	return strconv.Itoa(start) + "," + strconv.Itoa(count)
}
