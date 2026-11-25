package diffdoc

import "strconv"

// HunkRange formats one side of a unified-diff hunk header (the "start,count"
// portion), where start is the 1-based line at which the range begins on that
// side. It follows the GNU diff conventions:
//
//   - A count of exactly 1 is written as just the start line number (the
//     ",count" suffix is omitted): e.g. HunkRange(9, 1) → "9".
//   - A count of 0 (a pure insertion or deletion, where this side contributes
//     no lines) is written as "start-1,0": an empty range has no first line,
//     so its start is the line before it, after which the other side's lines
//     go. E.g. HunkRange(9, 0) → "8,0", and HunkRange(1, 0) → "0,0" for an
//     empty range at the top of the file or an empty side. Tools such as
//     patch(1) place the change using this start, so it must not be the
//     line that follows the range.
//   - Any other count is written as "start,count": e.g. HunkRange(9, 3) →
//     "9,3".
func HunkRange(start, count int) string {
	switch count {
	case 1:
		return strconv.Itoa(start)
	case 0:
		return strconv.Itoa(max(start-1, 0)) + ",0"
	default:
		return strconv.Itoa(start) + "," + strconv.Itoa(count)
	}
}
