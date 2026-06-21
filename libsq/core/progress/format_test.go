package progress_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/progress"
)

func TestGroupBarCounterText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current int64
		want    string
	}{
		{0, ""},
		{1, "1 item"},
		{2, "2 items"},
		{42, "42 items"},
		{1000, "1,000 items"},
		{1234567, "1,234,567 items"},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, progress.GroupBarCounterText(tc.current))
	}
}

func TestUnitCounterText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current int64
		unit    string
		want    string
	}{
		{0, "rec", "0 recs"},
		{1, "rec", "1 rec"},
		{2, "rec", "2 recs"},
		{1000, "rec", "1,000 recs"},
		{5, "", "5"},
		{0, "", "0"},
		{1234567, "row", "1,234,567 rows"},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, progress.UnitCounterText(tc.current, tc.unit))
	}
}

func TestUnitTotalCounterText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		current int64
		total   int64
		unit    string
		want    string
	}{
		{4, 16, "sheet", "4 / 16 sheets"},
		{1, 1, "sheet", "1 / 1 sheet"},
		{0, 0, "", "0 / 0"},
		{1000, 2000, "row", "1,000 / 2,000 rows"},
		{0, 10, "rec", "0 / 10 recs"},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, progress.UnitTotalCounterText(tc.current, tc.total, tc.unit))
	}
}

func TestTimeoutText(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		remaining time.Duration
		want      string
	}{
		{7 * time.Second, "timeout in 7s"},
		{90 * time.Second, "timeout in 1m30s"},
		{1500 * time.Millisecond, "timeout in 2s"},
		{0, "timeout in 0s"},
		{-500 * time.Millisecond, "timeout in 0s"},
		{-2 * time.Second, "timed out"},
		{-time.Hour, "timed out"},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, progress.TimeoutText(tc.remaining),
			"remaining=%s", tc.remaining)
	}
}

func TestMsgWidth(t *testing.T) {
	t.Parallel()

	// Shorter than max: padded with spaces to exactly msgMaxLen bytes.
	got := progress.MsgWidth("hello")
	require.Equal(t, progress.MsgMaxLen, len(got))
	require.True(t, strings.HasPrefix(got, "hello"))
	require.Equal(t, "hello", strings.TrimRight(got, " "))

	// Exactly msgMaxLen: returned unchanged.
	exact := strings.Repeat("x", progress.MsgMaxLen)
	require.Equal(t, exact, progress.MsgWidth(exact))

	// Longer than max: ellipsified to msgMaxLen runes (visual width).
	long := strings.Repeat("y", progress.MsgMaxLen*2)
	gotLong := progress.MsgWidth(long)
	require.Equal(t, progress.MsgMaxLen, utf8.RuneCountInString(gotLong))
	require.Contains(t, gotLong, "…")
}
